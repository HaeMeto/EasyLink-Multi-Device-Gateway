//go:build windows

package main

import (
 "context"
 "embed"
 "fmt"
 "io/fs"
 "log"
 "net/http"
 "os"
 "os/signal"
 gosync "sync"
 "syscall"
 "time"

 "easylink/gateway/internal/config"
 "easylink/gateway/internal/database"
 "easylink/gateway/internal/handlers"
 "easylink/gateway/internal/services"
)

//go:embed all:template
var templateFS embed.FS

//go:embed ui/*
var uiFS embed.FS

type jobRecorder struct {
 db *database.DB
}

func (j *jobRecorder) RecordJob(sdkNo int, sn string, action string, status string, request string, response string) error {
 _, err := j.db.Exec(
 `INSERT INTO jobs (sdk_no, sn, action, status, request, response, retry_count, created_at)
 VALUES (?, ?, ?, ?, ?, ?, 0, datetime('now'))`,
 sdkNo, sn, action, status, request, response,
 )
 return err
}

func main() {
 log.SetFlags(log.LstdFlags | log.Lshortfile)

 cfg, err := config.Load()
 if err != nil {
 log.Fatalf("config: %v", err)
 }

 db, err := database.Open(cfg.DBPath)
 if err != nil {
 log.Fatalf("database: %v", err)
 }
 defer db.Close()

 if err := db.Migrate(); err != nil {
 log.Fatalf("migrate: %v", err)
 }
 log.Println("database migrated ok")

 eventLogger := services.NewEventLogger(500)

 sync := services.NewSyncService(db, cfg.RootDeviceIniPath)
 if _, err := os.Stat(cfg.RootDeviceIniPath); os.IsNotExist(err) {
 os.WriteFile(cfg.RootDeviceIniPath, nil, 0644)
 }
 if err := sync.FullSync(); err != nil {
 log.Printf("sync warning: %v", err)
 }
 db.Exec("UPDATE devices SET enabled = 1 WHERE enabled = 0")

 sdkMgr := services.NewSdkManager(db, templateFS, cfg.InstancesPath, cfg.FServiceStartPort, sync, eventLogger)

 rowsAS, err := db.Query("SELECT sdk_no FROM sdk_instances")
 if err == nil {
 type autoRow struct { sdkNo int }
 var autoList []autoRow
 for rowsAS.Next() {
 var r autoRow
 if rowsAS.Scan(&r.sdkNo) == nil {
 autoList = append(autoList, r)
 }
 }
 rowsAS.Close()
 for _, r := range autoList {
 var devCount int
 db.QueryRow("SELECT COUNT(*) FROM devices WHERE sdk_no = ? AND enabled = 1", r.sdkNo).Scan(&devCount)
 if devCount > 0 {
 if err := sdkMgr.Start(r.sdkNo); err != nil {
 log.Printf("auto-start instance %d: %v", r.sdkNo, err)
 } else {
 log.Printf("auto-started instance %d", r.sdkNo)
 }
 }
 }
 }

 proxy := services.NewFServiceProxy()

 deviceLookup := func(sn string) (sdkNo int, port int, err error) {
 err = db.QueryRow(
 `SELECT d.sdk_no, COALESCE(i.port, 0)
 FROM devices d LEFT JOIN sdk_instances i ON d.sdk_no = i.sdk_no
 WHERE d.sn = ? AND d.enabled = 1`, sn,
 ).Scan(&sdkNo, &port)
 if err != nil {
 return 0, 0, fmt.Errorf("device %s not found: %w", sn, err)
 }
 if sdkNo == 0 {
 err = db.QueryRow(
 "SELECT sdk_no, port FROM sdk_instances WHERE status = 'RUNNING' ORDER BY sdk_no LIMIT 1",
 ).Scan(&sdkNo, &port)
 if err != nil {
 return 0, 0, fmt.Errorf("device %s not assigned and no instance available", sn)
 }
 }
 if port == 0 {
 return 0, 0, fmt.Errorf("device %s instance port not found", sn)
 }
 return
 }

 jr := &jobRecorder{db: db}
 queue := services.NewQueueManager(proxy, jr, deviceLookup, eventLogger)

 wd := services.NewWatchdog(cfg.WatchdogDuration(), db, sdkMgr, queue, proxy, eventLogger)

 h := &handlers.Handler{
 DB: db,
 SdkMgr: sdkMgr,
 Sync: sync,
 Queue: queue,
 Watchdog: wd,
 Logger: eventLogger,
 }

 mux := http.NewServeMux()

 mux.HandleFunc("/health", h.HandleHealth)

 mux.HandleFunc("GET /api/instances", h.HandleListInstances)
 mux.HandleFunc("POST /api/instances", h.HandleCreateInstance)
 mux.HandleFunc("POST /api/instances/{id}/start", h.HandleStartInstance)
 mux.HandleFunc("POST /api/instances/{id}/stop", h.HandleStopInstance)
 mux.HandleFunc("POST /api/instances/{id}/restart", h.HandleRestartInstance)
 mux.HandleFunc("DELETE /api/instances/{id}", h.HandleDeleteInstance)

 mux.HandleFunc("GET /api/devices", h.HandleListDevices)
 mux.HandleFunc("POST /api/devices", h.HandleCreateDevice)
 mux.HandleFunc("GET /api/devices/{id}", h.HandleGetDevice)
	mux.HandleFunc("PUT /api/devices/{id}", h.HandleUpdateDevice)
	mux.HandleFunc("POST /api/devices/{id}/toggle", h.HandleToggleDevice)
	mux.HandleFunc("DELETE /api/devices/{id}", h.HandleDeleteDevice)
 mux.HandleFunc("GET /api/devices/{id}/config", h.HandleGetDeviceConfig)
 mux.HandleFunc("PUT /api/devices/{id}/config", h.HandleUpdateDeviceConfig)
 mux.HandleFunc("DELETE /api/devices/{id}/config/{key}", h.HandleDeleteDeviceConfig)

 mux.HandleFunc("GET /api/devices/{sn}/info", h.HandleDeviceInfo)
 mux.HandleFunc("GET /api/devices/{sn}/scan/new", h.HandleScanlogNew)
 mux.HandleFunc("GET /api/devices/{sn}/scan/all", h.HandleScanlogAll)
 mux.HandleFunc("POST /api/devices/{sn}/scan/delete", h.HandleScanlogDelete)
 mux.HandleFunc("GET /api/devices/{sn}/scan/gps", h.HandleScanlogGPS)
 mux.HandleFunc("GET /api/devices/{sn}/users", h.HandleUserAll)
 mux.HandleFunc("POST /api/devices/{sn}/users", h.HandleUserSet)
 mux.HandleFunc("POST /api/devices/{sn}/users/batch", h.HandleUserSetAll)
 mux.HandleFunc("DELETE /api/devices/{sn}/users/{pin}", h.HandleUserDelete)
 mux.HandleFunc("DELETE /api/devices/{sn}/users", h.HandleUserDeleteAll)
 mux.HandleFunc("POST /api/devices/{sn}/time", h.HandleDeviceSetTime)
 mux.HandleFunc("POST /api/devices/{sn}/init", h.HandleDeviceInit)
 mux.HandleFunc("POST /api/devices/{sn}/deladmin", h.HandleDeviceDelAdmin)
 mux.HandleFunc("POST /api/devices/{sn}/log/del", h.HandleLogDel)

 mux.HandleFunc("POST /api/sync/reload", h.HandleSyncReload)
 mux.HandleFunc("GET /api/sync/status", h.HandleSyncStatus)
 mux.HandleFunc("GET /api/jobs", h.HandleJobs)
 mux.HandleFunc("GET /api/logs", h.HandleLogs)
 mux.HandleFunc("GET /api/logs/stream", h.HandleLogStream)

 uiContent, err := fs.Sub(uiFS, "ui")
 if err != nil {
 log.Printf("ui sub: %v", err)
 } else {
 mux.Handle("/", http.FileServer(http.FS(uiContent)))
 }

 ctx, cancel := context.WithCancel(context.Background())
 defer cancel()
 wd.Start(ctx)

 server := &http.Server{
 Addr: cfg.ListenAddr(),
 Handler: corsMiddleware(mux),
 }

 go func() {
 log.Printf("gateway listening on %s", cfg.ListenAddr())
 eventLogger.Log("system", fmt.Sprintf("Gateway started on %s", cfg.ListenAddr()))
 if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
 log.Fatalf("server: %v", err)
 }
 }()

 sigCh := make(chan os.Signal, 1)
 signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
 <-sigCh

 log.Println("shutting down...")
 eventLogger.Log("system", "shutting down")
 cancel()
 eventLogger.Close()

 runningSdkNos := sdkMgr.ListRunningSdkNos()
 var wg gosync.WaitGroup
 for _, sdkNo := range runningSdkNos {
 wg.Add(1)
 go func(sdkNo int) {
 defer wg.Done()
 if err := sdkMgr.Stop(sdkNo); err != nil {
 log.Printf("shutdown: stop instance %d: %v", sdkNo, err)
 }
 }(sdkNo)
 }
 wg.Wait()

 ctx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
 defer shutdownCancel()
 if err := server.Shutdown(ctx); err != nil {
 log.Printf("shutdown: server: %v", err)
 }
}

func corsMiddleware(next http.Handler) http.Handler {
 return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
 w.Header().Set("Access-Control-Allow-Origin", "*")
 w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
 w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
 if r.Method == http.MethodOptions {
 w.WriteHeader(http.StatusNoContent)
 return
 }
 next.ServeHTTP(w, r)
 })
}
