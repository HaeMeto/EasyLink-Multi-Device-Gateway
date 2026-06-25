package services

import (
 "context"
 "encoding/json"
 "fmt"
 "log"
 "strconv"
 "sync"
 "time"

 "easylink/gateway/internal/database"
 "easylink/gateway/internal/models"
)

type Syncer struct {
 easylinkDB *database.DB
 absenDB *database.DB
 proxy *FServiceProxy
 deviceLookup func(sn string) (sdkNo int, port int, err error)
 logger *EventLogger
 watchdog *Watchdog
}

func NewSyncer(easylinkDB *database.DB, absenDB *database.DB, proxy *FServiceProxy, deviceLookup func(sn string) (sdkNo int, port int, err error), logger *EventLogger, watchdog *Watchdog) *Syncer {
 return &Syncer{
 easylinkDB: easylinkDB,
 absenDB: absenDB,
 proxy: proxy,
 deviceLookup: deviceLookup,
 logger: logger,
 watchdog: watchdog,
 }
}

func (s *Syncer) Start(ctx context.Context) {
	go func() {
		for {
			interval := s.getSyncInterval()
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
				s.tick(ctx)
			}
		}
	}()
}

func (s *Syncer) getSyncInterval() time.Duration {
	var val string
	err := s.easylinkDB.QueryRow("SELECT value FROM config WHERE key = 'scanlog_sync_interval'").Scan(&val)
	if err != nil {
		val = "60"
	}
	sec, err := strconv.Atoi(val)
	if err != nil || sec < 5 {
		sec = 60
	}
	return time.Duration(sec) * time.Second
}

func (s *Syncer) tick(ctx context.Context) {
 type devRow struct {
 SN string
 }
 var devs []devRow

 rows, err := s.easylinkDB.Query("SELECT sn FROM devices WHERE enabled = 1 AND online = 1")
 if err != nil {
 log.Printf("syncer: query devices: %v", err)
 return
 }
 for rows.Next() {
 var d devRow
 if rows.Scan(&d.SN) == nil {
 devs = append(devs, d)
 }
 }
 rows.Close()

 sem := make(chan struct{}, 3)
 var wg sync.WaitGroup

 for _, d := range devs {
 select {
 case <-ctx.Done():
 wg.Wait()
 return
 default:
 }
 sdkNo, port, err := s.deviceLookup(d.SN)
 if err != nil {
 continue
 }
 wg.Add(1)
 go func(sn string, sdkNo int, port int) {
 defer wg.Done()
 sem <- struct{}{}
 defer func() { <-sem }()
 s.doDeviceSync(ctx, sdkNo, port, sn)
 }(d.SN, sdkNo, port)
 }

 wg.Wait()
}

func (s *Syncer) doDeviceSync(ctx context.Context, sdkNo int, port int, sn string) {
 select {
 case <-ctx.Done():
 return
 default:
 }

 if s.watchdog != nil && s.watchdog.IsDeviceInCooldown(sn) {
 return
 }

 var status string
	err := s.absenDB.QueryRow("SELECT scanlog_status FROM device_info WHERE sn = ?", sn).Scan(&status)
	if err == nil && status == "syncing" {
		return
	}

	infoData, err := s.proxy.DeviceInfo(port, sn)
	if err != nil {
		if s.logger != nil {
			s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s dev/info failed: %v", sdkNo, sn, err))
		}
		return
	}

	var devInfo models.DeviceInfoResponse
	if err := json.Unmarshal(infoData, &devInfo); err != nil {
		if s.logger != nil {
			s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s dev/info parse failed: %v", sdkNo, sn, err))
		}
		return
	}
	allPresensi := devInfo.GetAllPresensi()

	var cnt int
	err = s.absenDB.QueryRow("SELECT scanlog_count FROM device_info WHERE sn = ?", sn).Scan(&cnt)

	now := time.Now().Format("2006-01-02 15:04:05")
	s.absenDB.Exec("UPDATE device_info SET last_scan_check = ? WHERE sn = ?", now, sn)
	if err != nil {
		s.absenDB.Exec("INSERT OR IGNORE INTO device_info (sn, scanlog_count, user_count) VALUES (?, 0, 0)", sn)
	}

 if cnt == allPresensi {
 s.absenDB.Exec("UPDATE device_info SET scanlog_status = 'idle' WHERE sn = ?", sn)
 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s syncer idle: %d records", sdkNo, sn, cnt))
 }
 return
 }

	s.absenDB.Exec("UPDATE device_info SET scanlog_status = 'syncing' WHERE sn = ?", sn)

	var inserted int
	newData, ferr := s.proxy.ScanlogNew(port, sn)
	if ferr == nil {
		var nd struct {
			Data []models.ScanlogEntry `json:"Data"`
		}
		if json.Unmarshal(newData, &nd) == nil {
			for _, e := range nd.Data {
				res, execErr := s.absenDB.Exec(
					"INSERT OR IGNORE INTO scanlog (sn, scan_date, pin, verify_mode, io_mode, work_code) VALUES (?, ?, ?, ?, ?, ?)",
					e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
				)
				if execErr == nil {
					if n, _ := res.RowsAffected(); n > 0 {
						inserted++
					}
				}
			}
		}
 }

 if inserted == 0 && allPresensi > cnt {
 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s syncer WARNING: device=%d, db=%d, 0 inserted (duplicates?)", sdkNo, sn, allPresensi, cnt))
 }
 }

 if ferr != nil {
		if inserted > 0 {
			s.absenDB.Exec("UPDATE device_info SET scanlog_count = scanlog_count + ?, scanlog_status = 'idle' WHERE sn = ?", inserted, sn)
		} else {
			s.absenDB.Exec("UPDATE device_info SET scanlog_status = 'stale' WHERE sn = ?", sn)
		}
 if s.logger != nil {
 if inserted == 0 {
 s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s syncer WARNING: device=%d, db=%d, sync failed: %v", sdkNo, sn, allPresensi, cnt, ferr))
 }
 s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s syncer sync error: %v (inserted %d), device=%d, db=%d", sdkNo, sn, ferr, inserted, allPresensi, cnt))
 }
		return
	}

	var newTotal int
	s.absenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&newTotal)
	s.absenDB.Exec("UPDATE device_info SET scanlog_count = ?, scanlog_status = 'idle', last_scan_sync = ? WHERE sn = ?", newTotal, now, sn)
	if s.logger != nil {
		s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s syncer synced: +%d new (%d→%d), device=%d", sdkNo, sn, inserted, cnt, newTotal, allPresensi))
	}
}
