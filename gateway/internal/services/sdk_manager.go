package services

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"easylink/gateway/internal/database"
	"easylink/gateway/internal/models"
)

type SdkManager struct {
 db *database.DB
 template embed.FS
 instancesPath string
 startPort int
 syncService *SyncService
 logger *EventLogger
 mu sync.Mutex

 pendingSetDef map[int]bool
 pendingSetDefMu sync.Mutex
}

type InstanceStatus struct {
 Running bool `json:"running"`
 PID int `json:"pid"`
 PortOpen bool `json:"port_open"`
 HTTPOk bool `json:"http_ok"`
 Status string `json:"status"`
}

func NewSdkManager(db *database.DB, template embed.FS, instancesPath string, startPort int, syncService *SyncService, logger *EventLogger) *SdkManager {
 return &SdkManager{
 db: db,
 template: template,
 instancesPath: instancesPath,
 startPort: startPort,
 syncService: syncService,
 logger: logger,
 pendingSetDef: make(map[int]bool),
 }
}

func (m *SdkManager) Create(sdkNo int, port int) (*models.SdkInstance, error) {
 var exists int
 err := m.db.QueryRow("SELECT COUNT(*) FROM sdk_instances WHERE sdk_no = ?", sdkNo).Scan(&exists)
 if err != nil {
 return nil, fmt.Errorf("check sdk_no: %w", err)
 }
 if exists > 0 {
 return nil, fmt.Errorf("sdk_no %d already exists", sdkNo)
 }

 name := fmt.Sprintf("sdk-%d", sdkNo)
 dst := filepath.Join(m.instancesPath, name)

 if err := os.MkdirAll(dst, 0755); err != nil {
 return nil, fmt.Errorf("mkdir %s: %w", dst, err)
 }

 if err := m.extractTemplate(dst); err != nil {
 os.RemoveAll(dst)
 return nil, fmt.Errorf("extract template: %w", err)
 }

 if err := WriteSetDef(dst+"\\SetDef.fin", port, m.db); err != nil {
 os.RemoveAll(dst)
 return nil, fmt.Errorf("write SetDef.fin: %w", err)
 }

 deviceIniPath := filepath.Join(dst, "Device.ini")
 if m.syncService != nil {
 rootIni := m.syncService.RootIniPath()
 if err := os.Symlink(rootIni, deviceIniPath); err != nil {
 src, _ := os.ReadFile(rootIni)
 os.WriteFile(deviceIniPath, src, 0644)
 }
 } else {
 os.WriteFile(deviceIniPath, nil, 0644)
 }

 logDir := dst + "\\Log"
 os.MkdirAll(logDir, 0755)

 instance := &models.SdkInstance{
 SdkNo: sdkNo,
 Name: name,
 Path: dst,
 Port: port,
 Status: models.StatusStopped,
 }

 result, err := m.db.Exec(
 `INSERT INTO sdk_instances (sdk_no, name, path, port, pid, status, restart_count, created_at)
 VALUES (?, ?, ?, ?, 0, ?, 0, datetime('now'))`,
 sdkNo, name, dst, port, models.StatusStopped,
 )
 if err != nil {
 os.RemoveAll(dst)
 return nil, fmt.Errorf("insert instance: %w", err)
 }

 id, _ := result.LastInsertId()
 instance.ID = int(id)

 if m.logger != nil {
 m.logger.Log("instance", fmt.Sprintf("Created sdk-%d port=%d", sdkNo, port))
 }
 return instance, nil
}

func (m *SdkManager) Delete(sdkNo int) error {
 var inst models.SdkInstance
 err := m.db.QueryRow(
 "SELECT id, sdk_no, name, path, port, pid, status FROM sdk_instances WHERE sdk_no = ?",
 sdkNo,
 ).Scan(&inst.ID, &inst.SdkNo, &inst.Name, &inst.Path, &inst.Port, &inst.PID, &inst.Status)
 if err != nil {
 return fmt.Errorf("instance %d not found: %w", sdkNo, err)
 }

 if inst.Status == models.StatusRunning {
 if err := m.Stop(sdkNo); err != nil {
 return fmt.Errorf("stop before delete: %w", err)
 }
 }

 for i := 0; i < 3; i++ {
 err = os.RemoveAll(inst.Path)
 if err == nil {
 break
 }
 if i == 2 {
 return fmt.Errorf("cannot delete directory %s: %w", inst.Path, err)
 }
 time.Sleep(500 * time.Millisecond)
 }

 _, err = m.db.Exec("DELETE FROM sdk_instances WHERE sdk_no = ?", sdkNo)
 if err != nil {
 return fmt.Errorf("delete instance: %w", err)
 }

 m.db.Exec("UPDATE devices SET sdk_no = 0 WHERE sdk_no = ?", sdkNo)

 if m.logger != nil {
 m.logger.Log("instance", fmt.Sprintf("Deleted sdk-%d", sdkNo))
 }
 return nil
}

func (m *SdkManager) Start(sdkNo int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startLocked(sdkNo)
}

func (m *SdkManager) startLocked(sdkNo int) error {
 var inst models.SdkInstance
 err := m.db.QueryRow(
 "SELECT id, sdk_no, name, path, port, pid, status FROM sdk_instances WHERE sdk_no = ?",
 sdkNo,
 ).Scan(&inst.ID, &inst.SdkNo, &inst.Name, &inst.Path, &inst.Port, &inst.PID, &inst.Status)
 if err != nil {
 return fmt.Errorf("instance %d not found: %w", sdkNo, err)
 }

 if inst.Status == models.StatusRunning {
 if isProcessAlive(inst.PID) {
 return fmt.Errorf("instance %d already running", sdkNo)
 }
 }

 exePath := filepath.Join(inst.Path, "FService.exe")
 if _, err := os.Stat(exePath); os.IsNotExist(err) {
 return fmt.Errorf("FService.exe not found at %s", exePath)
 }

 cmd := exec.Command(exePath)
 cmd.Dir = inst.Path
 cmd.SysProcAttr = sysProcAttr

 if err := cmd.Start(); err != nil {
 return fmt.Errorf("start FService: %w", err)
 }

 pid := cmd.Process.Pid

 _, err = m.db.Exec(
 "UPDATE sdk_instances SET pid=?, status=?, restart_count=0 WHERE sdk_no=?",
 pid, models.StatusRunning, sdkNo,
 )
 if err != nil {
 return fmt.Errorf("update instance status: %w", err)
 }

 if m.logger != nil {
 m.logger.Log("instance", fmt.Sprintf("Starting sdk-%d PID=%d", sdkNo, pid))
 }
 return nil
}

func (m *SdkManager) Stop(sdkNo int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopLocked(sdkNo)
}

func (m *SdkManager) stopLocked(sdkNo int) error {
 var inst models.SdkInstance
 err := m.db.QueryRow(
 "SELECT id, sdk_no, name, path, port, pid, status FROM sdk_instances WHERE sdk_no = ?",
 sdkNo,
 ).Scan(&inst.ID, &inst.SdkNo, &inst.Name, &inst.Path, &inst.Port, &inst.PID, &inst.Status)
 if err != nil {
 return fmt.Errorf("instance %d not found: %w", sdkNo, err)
 }

 origPID := inst.PID
 origStatus := inst.Status

 if m.logger != nil {
 m.logger.Log("instance", fmt.Sprintf("Stopping sdk-%d", sdkNo))
 }

 _, err = m.db.Exec(
 "UPDATE sdk_instances SET pid=0, status=? WHERE sdk_no=?",
 models.StatusStopped, sdkNo,
 )
 if err != nil {
 if m.logger != nil {
 m.logger.Log("instance", fmt.Sprintf("stop sdk-%d: db update failed: %v", sdkNo, err))
 }
 return fmt.Errorf("update instance status: %w", err)
 }

 if origPID > 0 {
 killErr := forceKill(origPID, inst.Port)
 if killErr != nil {
 m.db.Exec(
 "UPDATE sdk_instances SET pid=?, status=? WHERE sdk_no=?",
 origPID, origStatus, sdkNo,
 )
 if m.logger != nil {
 m.logger.Log("instance", fmt.Sprintf("stop sdk-%d failed: %v", sdkNo, killErr))
 }
 return killErr
 }
 }

 if m.logger != nil {
 m.logger.Log("instance", fmt.Sprintf("Stopped sdk-%d", sdkNo))
 }
 return nil
}

func forceKill(pid int, port int) error {
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	cmd.SysProcAttr = sysProcAttr
	cmd.Run()

	if !waitProcessDead(pid, 10, 500*time.Millisecond) {
		for retry := 0; retry < 3; retry++ {
			if err := terminateProcess(pid); err == nil {
				time.Sleep(1 * time.Second)
				if !isProcessAlive(pid) {
					break
				}
			}
			if retry < 2 {
				time.Sleep(1 * time.Second)
			}
		}
		if isProcessAlive(pid) {
			return fmt.Errorf("process %d could not be killed after all attempts", pid)
		}
	}

	for i := 0; i < 10; i++ {
		conn, dErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
		if dErr != nil {
			return nil
		}
		conn.Close()
		if i == 9 {
			for retry := 0; retry < 5; retry++ {
				if err := terminateProcess(pid); err == nil {
					time.Sleep(1 * time.Second)
					if !isProcessAlive(pid) {
						break
					}
				}
				if retry < 4 {
					time.Sleep(1 * time.Second)
				}
			}
			for j := 0; j < 3; j++ {
				conn2, err2 := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
				if err2 != nil {
					return nil
				}
				conn2.Close()
				time.Sleep(1 * time.Second)
			}
			return fmt.Errorf("port %d still open after all attempts", port)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

func waitProcessDead(pid int, attempts int, delay time.Duration) bool {
 for i := 0; i < attempts; i++ {
 if !isProcessAlive(pid) {
 return true
 }
 if i < attempts-1 {
 time.Sleep(delay)
 }
 }
 return false
}

func (m *SdkManager) Restart(sdkNo int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.logger != nil {
		m.logger.Log("instance", fmt.Sprintf("Restarting sdk-%d", sdkNo))
	}
	if err := m.stopLocked(sdkNo); err != nil {
		return fmt.Errorf("restart stop: %w", err)
	}

	var inst models.SdkInstance
	m.db.QueryRow(
		"SELECT path, port FROM sdk_instances WHERE sdk_no = ?",
		sdkNo,
	).Scan(&inst.Path, &inst.Port)

	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", inst.Port), 1*time.Second)
		if err != nil {
			break
		}
		conn.Close()
		if i == 9 {
			return fmt.Errorf("port %d still in use after stop", inst.Port)
		}
		time.Sleep(500 * time.Millisecond)
	}

 ldbPath := filepath.Join(inst.Path, "db_temp.ldb")
 os.Remove(ldbPath)
 os.Remove(filepath.Join(inst.Path, "db_temp.laccdb"))
 os.Remove(filepath.Join(inst.Path, "db_temp.lock"))
 templateContent, err := m.template.ReadFile("template/db_temp.mdb")
 if err == nil {
 os.WriteFile(filepath.Join(inst.Path, "db_temp.mdb"), templateContent, 0644)
 }

 if err := WriteSetDef(filepath.Join(inst.Path, "SetDef.fin"), inst.Port, m.db); err != nil {
 if m.logger != nil {
 m.logger.Log("instance", fmt.Sprintf("sdk-%d rewrite SetDef.fin failed: %v", sdkNo, err))
 }
 }

 if err := m.startLocked(sdkNo); err != nil {
		return fmt.Errorf("restart start: %w", err)
	}

	time.Sleep(3 * time.Second)

	m.db.Exec(
 "UPDATE sdk_instances SET restart_count = restart_count + 1, last_restart = datetime('now') WHERE sdk_no = ?",
 sdkNo,
 )

 return nil
}

func (m *SdkManager) ListRunningSdkNos() []int {
	rows, err := m.db.Query("SELECT sdk_no FROM sdk_instances WHERE status IN (?, ?, ?, ?)", models.StatusRunning, models.StatusBusy, models.StatusBusyScanlog, models.StatusBusyUser)
 if err != nil {
 return nil
 }
 defer rows.Close()
 var result []int
 for rows.Next() {
 var sdkNo int
 if err := rows.Scan(&sdkNo); err != nil {
 continue
 }
 result = append(result, sdkNo)
 }
 return result
}

func (m *SdkManager) Status(sdkNo int) (*InstanceStatus, error) {
 var inst models.SdkInstance
 err := m.db.QueryRow(
 "SELECT id, sdk_no, name, path, port, pid, status FROM sdk_instances WHERE sdk_no = ?",
 sdkNo,
 ).Scan(&inst.ID, &inst.SdkNo, &inst.Name, &inst.Path, &inst.Port, &inst.PID, &inst.Status)
 if err != nil {
 return nil, fmt.Errorf("instance %d not found: %w", sdkNo, err)
 }

 st := &InstanceStatus{
 Status: inst.Status,
 PID: inst.PID,
 }

 st.Running = isProcessAlive(inst.PID)

 return st, nil
}

func (m *SdkManager) ListAll() ([]models.SdkInstance, error) {
 rows, err := m.db.Query(
 "SELECT id, sdk_no, name, path, port, pid, status, restart_count, COALESCE(last_restart,''), created_at FROM sdk_instances ORDER BY sdk_no",
 )
 if err != nil {
 return nil, fmt.Errorf("query instances: %w", err)
 }
 defer rows.Close()

 var instances []models.SdkInstance
 for rows.Next() {
 var inst models.SdkInstance
 if err := rows.Scan(&inst.ID, &inst.SdkNo, &inst.Name, &inst.Path, &inst.Port, &inst.PID, &inst.Status, &inst.RestartCount, &inst.LastRestart, &inst.CreatedAt); err != nil {
 return nil, fmt.Errorf("scan instance: %w", err)
 }
 instances = append(instances, inst)
 }
 return instances, rows.Err()
}

func (m *SdkManager) extractTemplate(dst string) error {
 return fs.WalkDir(m.template, "template", func(path string, d fs.DirEntry, err error) error {
 if err != nil {
 return err
 }

 relPath := strings.TrimPrefix(path, "template")
 relPath = strings.TrimPrefix(relPath, "/")
 relPath = strings.TrimPrefix(relPath, "\\")

 if relPath == "" {
 return nil
 }

 lower := strings.ToLower(relPath)
 skipFiles := map[string]bool{
 "device.ini": true,
 "setdef.fin": true,
 "db_temp.ldb": true,
 }
 baseName := strings.ToLower(filepath.Base(relPath))
 if skipFiles[baseName] {
 return nil
 }

 skipDirs := map[string]bool{"log": true}
 if d.IsDir() && skipDirs[lower] {
 return fs.SkipDir
 }

 target := filepath.Join(dst, relPath)

 if d.IsDir() {
 return os.MkdirAll(target, 0755)
 }

 src, err := m.template.Open(path)
 if err != nil {
 return fmt.Errorf("open embed %s: %w", path, err)
 }
 defer src.Close()

 dir := filepath.Dir(target)
 if err := os.MkdirAll(dir, 0755); err != nil {
 return err
 }

 dstFile, err := os.Create(target)
 if err != nil {
 return fmt.Errorf("create %s: %w", target, err)
 }
 defer dstFile.Close()

 if _, err := io.Copy(dstFile, src); err != nil {
 return fmt.Errorf("copy %s: %w", target, err)
 }

 return nil
 })
}

func (m *SdkManager) InjectSetDefAll() []string {
 var logs []string
 rows, err := m.db.Query("SELECT sdk_no, path, port, status FROM sdk_instances")
 if err != nil {
 return []string{"query instances: " + err.Error()}
 }
 defer rows.Close()

 type inst struct {
 sdkNo int
 path string
 port int
 status string
 }
 var instances []inst
 for rows.Next() {
 var i inst
 if rows.Scan(&i.sdkNo, &i.path, &i.port, &i.status) != nil {
 continue
 }
 instances = append(instances, i)
 }

 for _, inst := range instances {
 if inst.status == models.StatusBusyScanlog || inst.status == models.StatusBusyUser {
 if err := WriteSetDef(filepath.Join(inst.path, "SetDef.fin"), inst.port, m.db); err != nil {
 logs = append(logs, fmt.Sprintf("sdk-%d: rewrite SetDef.fin failed: %v", inst.sdkNo, err))
 } else {
 logs = append(logs, fmt.Sprintf("sdk-%d: SetDef.fin updated, pending restart after operation completes", inst.sdkNo))
 }
 m.MarkPendingSetDef(inst.sdkNo)
 continue
 }

 if err := WriteSetDef(filepath.Join(inst.path, "SetDef.fin"), inst.port, m.db); err != nil {
 logs = append(logs, fmt.Sprintf("sdk-%d: rewrite SetDef.fin failed: %v", inst.sdkNo, err))
 continue
 }

 shouldRestart := inst.status == models.StatusRunning || inst.status == models.StatusBusy
 shouldStart := inst.status == models.StatusStopped

 if shouldRestart {
 if m.logger != nil {
 m.logger.Log("instance", fmt.Sprintf("sdk-%d: SetDef.fin updated, restarting", inst.sdkNo))
 }
 if err := m.Restart(inst.sdkNo); err != nil {
 logs = append(logs, fmt.Sprintf("sdk-%d: restart failed: %v", inst.sdkNo, err))
 } else {
 logs = append(logs, fmt.Sprintf("sdk-%d: SetDef.fin updated + restarted", inst.sdkNo))
 }
 } else if shouldStart {
 if err := m.Start(inst.sdkNo); err != nil {
 logs = append(logs, fmt.Sprintf("sdk-%d: start failed: %v", inst.sdkNo, err))
 } else {
 logs = append(logs, fmt.Sprintf("sdk-%d: SetDef.fin updated + started", inst.sdkNo))
 }
 } else {
 logs = append(logs, fmt.Sprintf("sdk-%d: SetDef.fin updated", inst.sdkNo))
 }
 }

 return logs
}

func (m *SdkManager) MarkPendingSetDef(sdkNo int) {
 m.pendingSetDefMu.Lock()
 defer m.pendingSetDefMu.Unlock()
 m.pendingSetDef[sdkNo] = true
}

func (m *SdkManager) ConsumePendingSetDef(sdkNo int) bool {
 m.pendingSetDefMu.Lock()
 defer m.pendingSetDefMu.Unlock()
 pending := m.pendingSetDef[sdkNo]
 if pending {
 delete(m.pendingSetDef, sdkNo)
 }
 return pending
}
