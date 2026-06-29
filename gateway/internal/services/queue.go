package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"

	"easylink/gateway/internal/database"
	"easylink/gateway/internal/models"
)

type WorkerStatus string

const (
	WorkerIdle       WorkerStatus = "IDLE"
	WorkerRunning    WorkerStatus = "RUNNING"
	WorkerRestarting WorkerStatus = "RESTARTING"
	WorkerOffline    WorkerStatus = "OFFLINE"
	WorkerError      WorkerStatus = "ERROR"
	WorkerBusy       WorkerStatus = "BUSY"
)

var fastActions = map[string]bool{
 "dev/info": true,
 "dev/settime": true,
}

type jobRequest struct {
	action     string
	sn         string
	params     url.Values
	responseCh chan jobResponse
	cleanup    func()
}

type jobResponse struct {
 data json.RawMessage
 err error
}

type DeviceWorker struct {
	sdkNo   int
	port    int
	queue   chan jobRequest
	status  WorkerStatus
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	proxy   *FServiceProxy
	absenDB *database.DB
	sdkMgr  *SdkManager
	db      *database.DB
	jobDB   JobRecorder
	logger  *EventLogger
}

type JobRecorder interface {
 RecordJob(sdkNo int, sn string, action string, status string, request string, response string) error
}

type QueueManager struct {
	workers      map[int]*DeviceWorker
	mu           sync.RWMutex
	proxy        *FServiceProxy
	absenDB      *database.DB
	sdkMgr       *SdkManager
	db           *database.DB
	jobDB        JobRecorder
	deviceLookup func(sn string) (sdkNo int, port int, err error)
	logger       *EventLogger
}

func NewQueueManager(proxy *FServiceProxy, absenDB *database.DB, jobDB JobRecorder, deviceLookup func(sn string) (sdkNo int, port int, err error), logger *EventLogger, sdkMgr *SdkManager, db *database.DB) *QueueManager {
	return &QueueManager{
		workers:      make(map[int]*DeviceWorker),
		proxy:        proxy,
		absenDB:      absenDB,
		sdkMgr:       sdkMgr,
		db:           db,
		jobDB:        jobDB,
		deviceLookup: deviceLookup,
		logger:       logger,
	}
}

func (qm *QueueManager) Enqueue(sn string, action string, params url.Values) (json.RawMessage, error) {
	assignedSdkNo, assignedPort, err := qm.deviceLookup(sn)
	if err != nil {
		return nil, err
	}

	sdkNo, port, cleanup, err := ResolveSmartRoute(qm.db, qm.sdkMgr, qm.proxy, qm.logger, sn, assignedSdkNo, assignedPort, false)
	if err != nil {
		return nil, err
	}

	if fastActions[action] {
		var instStatus string
		if scanErr := qm.db.QueryRow("SELECT status FROM sdk_instances WHERE sdk_no = ?", sdkNo).Scan(&instStatus); scanErr == nil {
			if models.IsBusyStatus(instStatus) || instStatus == models.StatusError {
				return nil, fmt.Errorf("instance sdk %d is %s", sdkNo, instStatus)
			}
		}

		var data json.RawMessage
		switch action {
		case "dev/info":
			data, err = qm.proxy.DeviceInfo(port, sn)
		case "dev/settime":
			data, err = qm.proxy.DeviceSetTime(port, sn)
		}

		if cleanup != nil {
			cleanup()
		}

		reqJSON, _ := json.Marshal(params)
		respJSON, _ := json.Marshal(data)
		status := "DONE"
		if err != nil {
			status = "ERROR"
		}
		qm.jobDB.RecordJob(sdkNo, sn, action, status, string(reqJSON), string(respJSON))

		return data, err
	}

	w := qm.getOrCreateWorker(sdkNo, port)

	req := jobRequest{
		action:     action,
		sn:         sn,
		params:     params,
		responseCh: make(chan jobResponse, 1),
		cleanup:    cleanup,
	}

	w.mu.RLock()
	st := w.status
	w.mu.RUnlock()

	if st == WorkerRestarting || st == WorkerOffline || st == WorkerError || st == WorkerBusy {
		return nil, fmt.Errorf("worker for sdk %d is %s", sdkNo, st)
	}

	w.queue <- req
	resp := <-req.responseCh

	reqJSON, _ := json.Marshal(params)
	respJSON, _ := json.Marshal(resp.data)
	status := "DONE"
	if resp.err != nil {
		status = "ERROR"
	}
	qm.jobDB.RecordJob(sdkNo, sn, action, status, string(reqJSON), string(respJSON))

	return resp.data, resp.err
}

func (qm *QueueManager) getOrCreateWorker(sdkNo int, port int) *DeviceWorker {
 qm.mu.RLock()
 w, ok := qm.workers[sdkNo]
 qm.mu.RUnlock()
 if ok {
 return w
 }

 qm.mu.Lock()
 defer qm.mu.Unlock()

 w, ok = qm.workers[sdkNo]
 if ok {
 return w
 }

 ctx, cancel := context.WithCancel(context.Background())
	w = &DeviceWorker{
		sdkNo:   sdkNo,
		port:    port,
		queue:   make(chan jobRequest, 100),
		status:  WorkerIdle,
		ctx:     ctx,
		cancel:  cancel,
		proxy:   qm.proxy,
		absenDB: qm.absenDB,
		sdkMgr:  qm.sdkMgr,
		db:      qm.db,
		jobDB:   qm.jobDB,
		logger:  qm.logger,
	}
 qm.workers[sdkNo] = w

 go w.run()

 return w
}

func (qm *QueueManager) RemoveWorker(sdkNo int) {
 qm.mu.Lock()
 defer qm.mu.Unlock()
 if w, ok := qm.workers[sdkNo]; ok {
 w.cancel()
 delete(qm.workers, sdkNo)
 }
}

func (qm *QueueManager) PauseWorker(sdkNo int) {
 qm.mu.RLock()
 defer qm.mu.RUnlock()
 if w, ok := qm.workers[sdkNo]; ok {
 w.mu.Lock()
 w.status = WorkerRestarting
 w.mu.Unlock()
 w.drainQueue()
 }
}

func (qm *QueueManager) ResumeWorker(sdkNo int) {
 qm.mu.RLock()
 defer qm.mu.RUnlock()
 if w, ok := qm.workers[sdkNo]; ok {
 w.mu.Lock()
 w.status = WorkerIdle
 w.mu.Unlock()
 }
}

func (qm *QueueManager) SetWorkerStatus(sdkNo int, status WorkerStatus) {
 qm.mu.RLock()
 defer qm.mu.RUnlock()
 if w, ok := qm.workers[sdkNo]; ok {
 w.mu.Lock()
 w.status = status
 w.mu.Unlock()
 }
}

func (w *DeviceWorker) run() {
 for {
 select {
 case <-w.ctx.Done():
 return
 case req := <-w.queue:
 w.processJob(req)
 }
 }
}

func (w *DeviceWorker) processJob(req jobRequest) {
 w.mu.Lock()
 w.status = WorkerRunning
 w.mu.Unlock()

 defer func() {
 w.mu.Lock()
 if w.status == WorkerRunning {
 w.status = WorkerIdle
 }
 w.mu.Unlock()
 }()

 var data json.RawMessage
 var err error
 var skipResponse bool

 if w.logger != nil {
 w.logger.Log("proxy", fmt.Sprintf("%s → %s (sdk-%d)", req.sn, req.action, w.sdkNo))
 }

 switch req.action {
 case "dev/init":
 data, err = w.proxy.DeviceInit(w.port, req.sn)
 case "dev/deladmin":
 data, err = w.proxy.DeviceDelAdmin(w.port, req.sn)
 case "scanlog/new":
 data, err = w.proxy.ScanlogNew(w.port, req.sn)
 case "scanlog/all":
 limit := 0
 if l, ok := req.params["limit"]; ok && len(l) > 0 {
 fmt.Sscanf(l[0], "%d", &limit)
 }
 data, err = w.proxy.ScanlogAll(w.port, req.sn, limit)
 case "scanlog/del":
 data, err = w.proxy.ScanlogDel(w.port, req.sn)
 case "scanlog/gps":
 byDate := ""
 if d, ok := req.params["by_date"]; ok && len(d) > 0 {
 byDate = d[0]
 }
 data, err = w.proxy.ScanlogGPS(w.port, req.sn, byDate)
 case "user/all":
 limit := 0
 if l, ok := req.params["limit"]; ok && len(l) > 0 {
 fmt.Sscanf(l[0], "%d", &limit)
 }
 data, err = w.proxy.UserAll(w.port, req.sn, limit)
 case "user/set":
 data, err = w.proxy.UserSet(
 w.port, req.sn,
 req.params.Get("pin"), req.params.Get("nama"), req.params.Get("pwd"),
 req.params.Get("rfid"), req.params.Get("priv"), req.params.Get("tmp"),
 )
 case "user/set-all":
 data, err = w.proxy.UserSetAll(w.port, req.sn, req.params.Get("data"))
 case "user/del":
 data, err = w.proxy.UserDel(w.port, req.sn, req.params.Get("pin"))
 case "user/delall":
 data, err = w.proxy.UserDelAll(w.port, req.sn)
	case "log/del":
		data, err = w.proxy.LogDel(w.port, req.sn)
	case "scanlog/sync":
		w.db.Exec("UPDATE sdk_instances SET status = ? WHERE sdk_no = ?", models.StatusBusyScanlog, w.sdkNo)
		w.mu.Lock()
		w.status = WorkerBusy
		w.mu.Unlock()
 defer func() {
 w.db.Exec("UPDATE sdk_instances SET status = ? WHERE sdk_no = ?", models.StatusRunning, w.sdkNo)
 w.mu.Lock()
 w.status = WorkerIdle
 w.mu.Unlock()
 if w.logger != nil {
 w.logger.Log("proxy", fmt.Sprintf("%s operation: BUSY-SCANLOG -> RUNNING", req.sn))
 }
 if w.sdkMgr != nil && w.sdkMgr.ConsumePendingSetDef(w.sdkNo) {
 if w.logger != nil {
 w.logger.Log("instance", fmt.Sprintf("sdk-%d pending SetDef restart", w.sdkNo))
 }
 go w.sdkMgr.Restart(w.sdkNo)
 }
 }()
 if w.logger != nil {
 w.logger.Log("proxy", fmt.Sprintf("%s operation: BUSY-SCANLOG", req.sn))
 }
		data, err = w.proxy.SyncScanlog(w.absenDB, w.port, req.sn, w.logger)
	case "scanlog/sync-new":
		data, err = w.proxy.SyncScanlogNew(w.absenDB, w.port, req.sn, w.logger)
	case "user/sync-full":
		limit := 0
		if l, ok := req.params["limit"]; ok && len(l) > 0 {
			fmt.Sscanf(l[0], "%d", &limit)
		}
		startedMsg, _ := json.Marshal(map[string]interface{}{"status": "started", "message": "Sync in progress"})
		req.responseCh <- jobResponse{data: json.RawMessage(startedMsg), err: nil}
		skipResponse = true
		w.db.Exec("UPDATE sdk_instances SET status = ? WHERE sdk_no = ?", models.StatusBusyUser, w.sdkNo)
		w.mu.Lock()
		w.status = WorkerBusy
		w.mu.Unlock()
 defer func() {
 w.db.Exec("UPDATE sdk_instances SET status = ? WHERE sdk_no = ?", models.StatusRunning, w.sdkNo)
 w.mu.Lock()
 w.status = WorkerIdle
 w.mu.Unlock()
 if w.logger != nil {
 w.logger.Log("proxy", fmt.Sprintf("%s operation: BUSY-SCANUSER -> RUNNING", req.sn))
 }
 if w.sdkMgr != nil && w.sdkMgr.ConsumePendingSetDef(w.sdkNo) {
 if w.logger != nil {
 w.logger.Log("instance", fmt.Sprintf("sdk-%d pending SetDef restart", w.sdkNo))
 }
 go w.sdkMgr.Restart(w.sdkNo)
 }
 }()
 if w.logger != nil {
 w.logger.Log("proxy", fmt.Sprintf("%s operation: BUSY-SCANUSER", req.sn))
 }
 data, err = w.proxy.SyncUsersFull(w.absenDB, w.port, req.sn, limit, w.sdkNo, w.logger)
		if errors.Is(err, ErrFServiceBusy) {
			if w.logger != nil {
				w.logger.Log("proxy", fmt.Sprintf("%s user sync busy → mitigation", req.sn))
			}
			data, err = MitigateUserSyncBusy(w.proxy, w.absenDB, w.sdkMgr, w.db, req.sn, limit, w.logger)
		}
	default:
 err = fmt.Errorf("unknown action: %s", req.action)
 }

 status := "DONE"
 if err != nil {
 status = "ERROR"
 }
	if w.logger != nil {
		w.logger.Log("proxy", fmt.Sprintf("%s ← %s %s", req.sn, req.action, status))
	}

	if req.cleanup != nil {
		req.cleanup()
	}

	if !skipResponse {
 req.responseCh <- jobResponse{data: data, err: err}
 }
}

func (w *DeviceWorker) drainQueue() {
 for {
 select {
 case req := <-w.queue:
 req.responseCh <- jobResponse{err: fmt.Errorf("worker restarting")}
 default:
 return
 }
 }
}
