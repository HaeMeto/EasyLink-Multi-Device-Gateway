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
	easylinkDB   *database.DB
	absenDB      *database.DB
	proxy        *FServiceProxy
	deviceLookup func(sn string) (sdkNo int, port int, err error)
	logger       *EventLogger
	watchdog     *Watchdog
	sdkMgr       *SdkManager
}

func NewSyncer(easylinkDB *database.DB, absenDB *database.DB, proxy *FServiceProxy, deviceLookup func(sn string) (sdkNo int, port int, err error), logger *EventLogger, watchdog *Watchdog, sdkMgr *SdkManager) *Syncer {
	return &Syncer{
		easylinkDB:   easylinkDB,
		absenDB:      absenDB,
		proxy:        proxy,
		deviceLookup: deviceLookup,
		logger:       logger,
		watchdog:     watchdog,
		sdkMgr:       sdkMgr,
	}
}

func (s *Syncer) Start(ctx context.Context) {
 go func() {
 defer func() {
 if r := recover(); r != nil {
 log.Printf("syncer panic: %v", r)
 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("PANIC recovered: %v", r))
 }
 }
 }()
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

	rows, err := s.easylinkDB.Query("SELECT sn FROM devices WHERE enabled = 1")
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

 log.Printf("syncer tick: %d enabled devices", len(devs))
 if len(devs) == 0 {
 if s.logger != nil {
 s.logger.Log("syncer", "tick: 0 devices found, nothing to sync")
 }
 }

 sem := make(chan struct{}, 3)
		var wg sync.WaitGroup

		for _, d := range devs {
			select {
			case <-ctx.Done():
				wg.Wait()
				return
			default:
			}
			wg.Add(1)
			go func(sn string) {
		defer wg.Done()
		sem <- struct{}{}
		defer func() { <-sem }()
		s.doDeviceSync(ctx, sn)
	}(d.SN)
 }

 wg.Wait()
}

func (s *Syncer) doDeviceSync(ctx context.Context, sn string) {
	select {
	case <-ctx.Done():
		return
	default:
	}

 if s.watchdog != nil && s.watchdog.IsDeviceInCooldown(sn) {
 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("skip %s: in cooldown", sn))
 }
 return
 }

	var status string
	err := s.absenDB.QueryRow("SELECT scanlog_status FROM device_info WHERE sn = ?", sn).Scan(&status)
 if err == nil && status == "syncing" {
 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("skip %s: scanlog_status='syncing'", sn))
 }
 return
 }

 assignedSdkNo, assignedPort, err := s.deviceLookup(sn)
 if err != nil {
 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("skip %s: deviceLookup failed: %v", sn, err))
 }
 return
 }

	sdkNo, port, cleanup, err := ResolveSmartRoute(s.easylinkDB, s.sdkMgr, s.proxy, s.logger, sn, assignedSdkNo, assignedPort, true)
	if err != nil {
		if s.logger != nil {
			s.logger.Log("syncer", fmt.Sprintf("syncer skip %s: smart route failed: %v", sn, err))
		}
		return
	}
	if cleanup != nil {
		defer cleanup()
	}

	var instStatus string
	if err := s.easylinkDB.QueryRow("SELECT status FROM sdk_instances WHERE sdk_no = ?", sdkNo).Scan(&instStatus); err == nil {
		if models.IsBusyStatus(instStatus) || instStatus == "ERROR" {
			if s.logger != nil {
				s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s syncer skip: instance status=%s", sdkNo, sn, instStatus))
			}
			return
		}
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
 if !devInfo.Result {
 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s dev/info: Result=false, skipping", sdkNo, sn))
 }
 return
 }
 allPresensi := devInfo.GetAllPresensi()
 newPresensi := devInfo.GetNewPresensi()

	if s.logger != nil {
		s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s dev/info: all=%d new=%d", sdkNo, sn, allPresensi, newPresensi))
		raw := string(infoData)
		if len(raw) > 300 {
			raw = raw[:300] + "..."
		}
		s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s dev/info raw: %s", sdkNo, sn, raw))
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	s.absenDB.Exec("INSERT OR IGNORE INTO device_info (sn, scanlog_count, user_count) VALUES (?, 0, 0)", sn)
	s.absenDB.Exec("UPDATE device_info SET last_scan_check = ? WHERE sn = ?", now, sn)

	if newPresensi > 0 {
		if s.logger != nil {
			s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s new-presensi=%d, fast path", sdkNo, sn, newPresensi))
		}

		s.easylinkDB.Exec("UPDATE sdk_instances SET status = ? WHERE sdk_no = ?", models.StatusBusyScanlog, sdkNo)
		defer func() {
			s.easylinkDB.Exec("UPDATE sdk_instances SET status = ? WHERE sdk_no = ?", models.StatusRunning, sdkNo)
		}()

		s.absenDB.Exec("UPDATE device_info SET scanlog_status = 'syncing' WHERE sn = ?", sn)

		newData, ferr := s.proxy.ScanlogNew(port, sn)
		if ferr != nil {
			s.absenDB.Exec("UPDATE device_info SET scanlog_status = 'stale' WHERE sn = ?", sn)
			if s.logger != nil {
				s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s fast path error: %v", sdkNo, sn, ferr))
			}
			return
		}

		var nd struct {
			Data []models.ScanlogEntry `json:"Data"`
		}
		if err := json.Unmarshal(newData, &nd); err != nil {
			s.absenDB.Exec("UPDATE device_info SET scanlog_status = 'stale' WHERE sn = ?", sn)
			if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s scanlog/new parse failed: %v", sdkNo, sn, err))
 }
 return
 }

 var totalInserted int
 for _, e := range nd.Data {
			res, execErr := s.absenDB.Exec(
				"INSERT OR IGNORE INTO scanlog (sn, scan_date, pin, verify_mode, io_mode, work_code) VALUES (?, ?, ?, ?, ?, ?)",
				e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
			)
			if execErr == nil {
				if n, _ := res.RowsAffected(); n > 0 {
					totalInserted++
				}
			}
		}

		var newTotal int
		s.absenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&newTotal)
		s.absenDB.Exec("UPDATE device_info SET scanlog_count = ?, scanlog_status = 'idle', last_scan_sync = ? WHERE sn = ?", newTotal, now, sn)
		if s.logger != nil {
			s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s fast path done: +%d new, total=%d, device=%d", sdkNo, sn, totalInserted, newTotal, allPresensi))
		}
		return
	}

	var localCount int
	s.absenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&localCount)
	if localCount == allPresensi {
		s.absenDB.Exec("UPDATE device_info SET scanlog_status = 'idle' WHERE sn = ?", sn)
		if s.logger != nil {
			s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s syncer idle: %d records", sdkNo, sn, localCount))
		}
 return
 }

 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s new-presensi=0, gap=%d, using full pagination", sdkNo, sn, allPresensi-localCount))
 }

 var totalInserted int

 s.easylinkDB.Exec("UPDATE sdk_instances SET status = ? WHERE sdk_no = ?", models.StatusBusyScanlog, sdkNo)
 defer func() {
 s.easylinkDB.Exec("UPDATE sdk_instances SET status = ? WHERE sdk_no = ?", models.StatusRunning, sdkNo)
 }()

 s.absenDB.Exec("UPDATE device_info SET scanlog_status = 'syncing' WHERE sn = ?", sn)

 pagingData, ferr := s.proxy.ScanlogAllFull(port, sn, 100, s.logger)
 if ferr != nil {
 s.absenDB.Exec("UPDATE device_info SET scanlog_status = 'stale' WHERE sn = ?", sn)
 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s full pagination error: %v", sdkNo, sn, ferr))
 }
 return
 }

 var pagingResp models.ScanlogPagingResponse
 if err := json.Unmarshal(pagingData, &pagingResp); err != nil {
 s.absenDB.Exec("UPDATE device_info SET scanlog_status = 'stale' WHERE sn = ?", sn)
 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s pagination parse failed: %v", sdkNo, sn, err))
 }
 return
 }

 for _, e := range pagingResp.Data {
 res, execErr := s.absenDB.Exec(
 "INSERT OR IGNORE INTO scanlog (sn, scan_date, pin, verify_mode, io_mode, work_code) VALUES (?, ?, ?, ?, ?, ?)",
 e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
 )
 if execErr == nil {
 if n, _ := res.RowsAffected(); n > 0 {
 totalInserted++
 }
 }
 }

 var newTotal int
 s.absenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&newTotal)
 s.absenDB.Exec("UPDATE device_info SET scanlog_count = ?, scanlog_status = 'idle', last_scan_sync = ? WHERE sn = ?", newTotal, now, sn)
 if s.logger != nil {
 s.logger.Log("syncer", fmt.Sprintf("sdk-%d %s syncer synced: +%d new, total=%d, device=%d", sdkNo, sn, totalInserted, newTotal, allPresensi))
 }
}
