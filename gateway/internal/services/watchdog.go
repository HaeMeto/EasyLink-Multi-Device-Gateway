package services

import (
 "bytes"
 "context"
 "encoding/json"
 "fmt"
 "io"
 "log"
 "net"
 "net/http"
 "os"
 "path/filepath"
 "sync"
 "time"

 "easylink/gateway/internal/database"
)

var errBusy = fmt.Errorf("busy")

const busyCooldownDuration = 60 * time.Second

type HealthReport struct {
	Total     int              `json:"total"`
	Running   int              `json:"running"`
	Stopped   int              `json:"stopped"`
	Error     int              `json:"error"`
	Instances []HealthInstance `json:"instances"`
}

type HealthInstance struct {
	SdkNo         int    `json:"sdk_no"`
	Status        string `json:"status"`
	PID           int    `json:"pid"`
	Port          int    `json:"port"`
	Alive         bool   `json:"alive"`
	PortOpen      bool   `json:"port_open"`
	HTTPOk        bool   `json:"http_ok"`
	DevicesOnline int    `json:"devices_online"`
	DevicesTotal  int    `json:"devices_total"`
}

type Watchdog struct {
 tickCount int
 interval time.Duration
 db *database.DB
 sdkMgr *SdkManager
 queueMgr *QueueManager
 proxy *FServiceProxy
 logger *EventLogger
 instanceFailCount map[int]int
 busyCooldown map[string]time.Time
 cooldownMu sync.RWMutex
}

func NewWatchdog(interval time.Duration, db *database.DB, sdkMgr *SdkManager, queueMgr *QueueManager, proxy *FServiceProxy, logger *EventLogger) *Watchdog {
 return &Watchdog{
 interval: interval,
 db: db,
 sdkMgr: sdkMgr,
 queueMgr: queueMgr,
 proxy: proxy,
 logger: logger,
 instanceFailCount: make(map[int]int),
 busyCooldown: make(map[string]time.Time),
 }
}

func (w *Watchdog) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.tick()
			}
		}
	}()
}

type runningInstance struct {
	sdkNo int
	port  int
	pid   int
	path  string
}

func (w *Watchdog) tick() {
	w.tickCount++

	instances := w.queryRunningInstances()
	if len(instances) == 0 {
		return
	}

	for _, inst := range instances {
		w.checkInstanceInfra(inst.sdkNo, inst.port, inst.pid, inst.path)
	}

	for _, inst := range instances {
		w.checkDevicesForInstance(inst.sdkNo, inst.port)
	}
}

func (w *Watchdog) queryRunningInstances() []runningInstance {
	rows, err := w.db.Query(
		"SELECT sdk_no, port, pid, path FROM sdk_instances WHERE status = ?",
		"RUNNING",
	)
	if err != nil {
		log.Printf("watchdog: query instances: %v", err)
		return nil
	}
	defer rows.Close()

	var instances []runningInstance
	for rows.Next() {
		var r runningInstance
		if err := rows.Scan(&r.sdkNo, &r.port, &r.pid, &r.path); err != nil {
			log.Printf("watchdog: scan: %v", err)
			return nil
		}
		instances = append(instances, r)
	}
	return instances
}

func (w *Watchdog) checkInstanceInfra(sdkNo int, port int, pid int, path string) {
	pidAlive := pid > 0 && isProcessAlive(pid)
	if !pidAlive {
		portOpen := w.checkPort(port) == nil
		if portOpen {
			newPID, findErr := findPIDByPort(port)
			if findErr == nil && newPID > 0 {
				w.db.Exec("UPDATE sdk_instances SET pid = ? WHERE sdk_no = ?", newPID, sdkNo)
				log.Printf("watchdog: [instance] sdk-%d pid refreshed %d->%d", sdkNo, pid, newPID)
				if w.logger != nil {
					w.logger.Log("watchdog", fmt.Sprintf("[instance] sdk-%d pid refreshed %d->%d", sdkNo, pid, newPID))
				}
				return
			}
		}
		w.instanceFailCount[sdkNo]++
		log.Printf("watchdog: [instance] sdk-%d unhealthy: pid=%d alive=%v port=%d open=%v [fail %d/5]", sdkNo, pid, pidAlive, port, portOpen, w.instanceFailCount[sdkNo])
		if w.logger != nil {
			w.logger.Log("watchdog", fmt.Sprintf("[instance] sdk-%d unhealthy: pid dead [fail %d/5]", sdkNo, w.instanceFailCount[sdkNo]))
		}
		if w.instanceFailCount[sdkNo] >= 5 {
			w.recoverInstance(sdkNo)
		}
		return
	}

	if err := w.checkPort(port); err != nil {
		w.instanceFailCount[sdkNo]++
		log.Printf("watchdog: [instance] sdk-%d unhealthy: port %d closed [fail %d/5]", sdkNo, port, w.instanceFailCount[sdkNo])
		if w.logger != nil {
			w.logger.Log("watchdog", fmt.Sprintf("[instance] sdk-%d unhealthy: port closed [fail %d/5]", sdkNo, w.instanceFailCount[sdkNo]))
		}
		if w.instanceFailCount[sdkNo] >= 5 {
			w.recoverInstance(sdkNo)
		}
		return
	}

	if err := w.checkLDBLock(path); err != nil {
		w.instanceFailCount[sdkNo]++
		log.Printf("watchdog: [instance] sdk-%d unhealthy: ldb [fail %d/5]", sdkNo, w.instanceFailCount[sdkNo])
		if w.logger != nil {
			w.logger.Log("watchdog", fmt.Sprintf("[instance] sdk-%d unhealthy: ldb [fail %d/5]", sdkNo, w.instanceFailCount[sdkNo]))
		}
		if w.instanceFailCount[sdkNo] >= 5 {
			w.recoverInstance(sdkNo)
		}
		return
	}

	w.instanceFailCount[sdkNo] = 0
}

func (w *Watchdog) checkDevicesForInstance(sdkNo int, port int) {
	rows, err := w.db.Query(
		"SELECT id, sn, online, fail_count, COALESCE(last_offline,'') FROM devices WHERE sdk_no = ? AND enabled = 1",
		sdkNo,
	)
	if err != nil {
		return
	}

	type devRow struct {
		id          int
		sn          string
		online      int
		failCount   int
		lastOffline string
	}

	var devices []devRow
	for rows.Next() {
		var d devRow
		if err := rows.Scan(&d.id, &d.sn, &d.online, &d.failCount, &d.lastOffline); err != nil {
			rows.Close()
			return
		}
		devices = append(devices, d)
	}
	rows.Close()

 for _, d := range devices {
 if d.online == 0 && d.lastOffline != "" {
 t, parseErr := time.Parse("2006-01-02 15:04:05", d.lastOffline)
 if parseErr == nil && time.Since(t) < 30*time.Minute {
 continue
 }
 w.db.Exec("UPDATE devices SET online = 1, fail_count = 0 WHERE id = ?", d.id)
 }

 if w.IsDeviceInCooldown(d.sn) {
 continue
 }

 err := w.checkDeviceHealth(port, d.sn)
 if err == nil {
			w.db.Exec("UPDATE devices SET fail_count = 0, online = 1, last_offline = '' WHERE id = ?", d.id)
			if d.failCount > 0 {
				log.Printf("watchdog: [device] sn=%s dev=%d sdk=%d recovered (was fail %d)", d.sn, d.id, sdkNo, d.failCount)
				if w.logger != nil {
					w.logger.Log("watchdog", fmt.Sprintf("[device] sdk-%d %s recovered (was fail %d)", sdkNo, d.sn, d.failCount))
				}
			} else if d.online == 0 {
				log.Printf("watchdog: [device] sn=%s dev=%d sdk=%d back online (retry)", d.sn, d.id, sdkNo)
				if w.logger != nil {
					w.logger.Log("watchdog", fmt.Sprintf("[device] sdk-%d %s back online (retry)", sdkNo, d.sn))
				}
			}
 } else if err == errBusy {
 w.MarkDeviceBusy(d.sn)
 log.Printf("watchdog: [device] sn=%s dev=%d sdk=%d busy", d.sn, d.id, sdkNo)
			if w.logger != nil {
				w.logger.Log("watchdog", fmt.Sprintf("[device] sdk-%d %s busy", sdkNo, d.sn))
			}
		} else {
			newCount := d.failCount + 1
			w.db.Exec("UPDATE devices SET fail_count = ? WHERE id = ?", newCount, d.id)
			if newCount >= 5 {
				w.db.Exec("UPDATE devices SET online = 0, last_offline = datetime('now') WHERE id = ?", d.id)
				log.Printf("watchdog: [device] sn=%s dev=%d sdk=%d OFFLINE after 5 failures", d.sn, d.id, sdkNo)
				if w.logger != nil {
					w.logger.Log("watchdog", fmt.Sprintf("[device] sdk-%d %s OFFLINE after 5 failures", sdkNo, d.sn))
				}
			} else {
				log.Printf("watchdog: [device] sn=%s dev=%d sdk=%d unhealthy: %s [fail %d/5]", d.sn, d.id, sdkNo, err, newCount)
				if w.logger != nil {
					w.logger.Log("watchdog", fmt.Sprintf("[device] sdk-%d %s unhealthy: %s [fail %d/5]", sdkNo, d.sn, err, newCount))
				}
			}
		}
	}
}

func (w *Watchdog) checkDeviceHealth(port int, sn string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	urlStr := fmt.Sprintf("http://127.0.0.1:%d/dev/info", port)
	reqBody := bytes.NewBufferString("sn=" + sn)
	resp, err := client.Post(urlStr, "application/x-www-form-urlencoded", reqBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST %s sn=%s: status %d", urlStr, sn, resp.StatusCode)
	}

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return readErr
	}

	var fResp struct {
		Result      bool `json:"Result"`
		MessageCode int  `json:"message_code"`
	}
	if json.Unmarshal(bodyBytes, &fResp) == nil {
		if !fResp.Result && fResp.MessageCode == 3 {
			return errBusy
		}
	}

	return nil
}

func (w *Watchdog) IsDeviceInCooldown(sn string) bool {
 w.cooldownMu.RLock()
 defer w.cooldownMu.RUnlock()
 t, ok := w.busyCooldown[sn]
 if !ok {
 return false
 }
 return time.Since(t) < busyCooldownDuration
}

func (w *Watchdog) MarkDeviceBusy(sn string) {
 w.cooldownMu.Lock()
 defer w.cooldownMu.Unlock()
 w.busyCooldown[sn] = time.Now()
}

func (w *Watchdog) checkPort(port int) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 3*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func (w *Watchdog) checkLDBLock(path string) error {
	ldbPath := filepath.Join(path, "db_temp.ldb")
	if _, err := os.Stat(ldbPath); os.IsNotExist(err) {
		return nil
	}
	f, err := os.OpenFile(ldbPath, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("lock file: %w", err)
	}
	f.Close()
	return nil
}

func (w *Watchdog) recoverInstance(sdkNo int) error {
	log.Printf("watchdog: recovering instance %d", sdkNo)
	if w.logger != nil {
		w.logger.Log("watchdog", fmt.Sprintf("Recovering sdk-%d", sdkNo))
	}

	w.queueMgr.PauseWorker(sdkNo)

	w.db.Exec("UPDATE sdk_instances SET status = 'ERROR' WHERE sdk_no = ?", sdkNo)

	if err := w.sdkMgr.Restart(sdkNo); err != nil {
		w.queueMgr.ResumeWorker(sdkNo)
		if w.logger != nil {
			w.logger.Log("watchdog", fmt.Sprintf("Recover sdk-%d failed: %v", sdkNo, err))
		}
		return fmt.Errorf("restart: %w", err)
	}

	w.queueMgr.ResumeWorker(sdkNo)

	delete(w.instanceFailCount, sdkNo)

	log.Printf("watchdog: instance %d recovered", sdkNo)
	if w.logger != nil {
		w.logger.Log("watchdog", fmt.Sprintf("Recovered sdk-%d", sdkNo))
	}
	return nil
}

func (w *Watchdog) GetHealthReport() HealthReport {
	report := HealthReport{}

	rows, err := w.db.Query(
		"SELECT sdk_no, port, pid, status FROM sdk_instances ORDER BY sdk_no",
	)
	if err != nil {
		return report
	}

	type instRow struct {
		sdkNo  int
		port   int
		pid    int
		status string
	}
	var instances []instRow
	for rows.Next() {
		var r instRow
		if err := rows.Scan(&r.sdkNo, &r.port, &r.pid, &r.status); err != nil {
			continue
		}
		instances = append(instances, r)
	}
	rows.Close()

	for _, inst := range instances {
		hi := HealthInstance{
			SdkNo:  inst.sdkNo,
			Port:   inst.port,
			PID:    inst.pid,
			Status: inst.status,
		}
		if hi.Status == "RUNNING" {
			hi.Alive = isProcessAlive(hi.PID)
			hi.PortOpen = w.checkPort(hi.Port) == nil
			w.db.QueryRow(
				"SELECT COUNT(*) FROM devices WHERE sdk_no = ? AND enabled = 1",
				hi.SdkNo,
			).Scan(&hi.DevicesTotal)
			w.db.QueryRow(
				"SELECT COUNT(*) FROM devices WHERE sdk_no = ? AND enabled = 1 AND online = 1",
				hi.SdkNo,
			).Scan(&hi.DevicesOnline)
			hi.HTTPOk = hi.DevicesOnline > 0
		}
		report.Instances = append(report.Instances, hi)
	}

	for _, h := range report.Instances {
		report.Total++
		switch h.Status {
		case "RUNNING":
			report.Running++
		case "STOPPED":
			report.Stopped++
		case "ERROR":
			report.Error++
		}
	}

	return report
}
