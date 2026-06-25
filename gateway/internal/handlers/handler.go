package handlers

import (
 "encoding/json"
 "errors"
 "net/http"
 "strconv"
 "strings"
 "time"

 "easylink/gateway/internal/database"
 "easylink/gateway/internal/services"
)

type testDeviceInfoRequest struct {
	SN    string `json:"sn"`
	SdkNo int    `json:"sdk_no"`
}

type Handler struct {
	DB       *database.DB
	AbsenDB  *database.DB
	Proxy    *services.FServiceProxy
	SdkMgr   *services.SdkManager
	Sync     *services.SyncService
	Queue    *services.QueueManager
	Watchdog *services.Watchdog
	Logger   *services.EventLogger
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v interface{}) {
 w.Header().Set("Content-Type", "application/json")
 w.WriteHeader(status)
 json.NewEncoder(w).Encode(v)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
 h.writeJSON(w, status, map[string]string{"error": msg})
}

func extractPathParam(r *http.Request, position int) string {
 parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
 if position < len(parts) {
 return parts[position]
 }
 return ""
}

func parseIntParam(r *http.Request, key string) int {
 v := r.URL.Query().Get(key)
 if v == "" {
 return 0
 }
 n, _ := strconv.Atoi(v)
 return n
}

func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
 report := h.Watchdog.GetHealthReport()
 resp := map[string]interface{}{
 "status": "ok",
 "health": report,
 }
 h.writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleDeviceInfo(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }
 data, err := h.smartDeviceInfo(sn)
 if errors.Is(err, services.ErrFServiceBusy) {
 if data != nil {
 h.writeRawJSON(w, http.StatusServiceUnavailable, data)
 } else {
 h.writeError(w, http.StatusServiceUnavailable, "device busy")
 }
 return
 }
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) resolveDevice(sn string) (sdkNo int, port int, err error) {
 err = h.DB.QueryRow(
 "SELECT d.sdk_no, COALESCE(i.port, 0) FROM devices d LEFT JOIN sdk_instances i ON d.sdk_no = i.sdk_no WHERE d.sn = ? AND d.enabled = 1",
 sn,
 ).Scan(&sdkNo, &port)
 if err != nil {
 return 0, 0, errors.New("device not found")
 }
 if sdkNo == 0 {
 err = h.DB.QueryRow(
 "SELECT sdk_no, port FROM sdk_instances WHERE status='RUNNING' ORDER BY sdk_no LIMIT 1",
 ).Scan(&sdkNo, &port)
 if err != nil {
 return 0, 0, errors.New("no running SDK instances")
 }
 }
 if port == 0 {
 return 0, 0, errors.New("port not found")
 }
 return sdkNo, port, nil
}

type altInstance struct {
 sdkNo int
 port int
}

func (h *Handler) queryAlternatePorts(excludeSdkNo int) []altInstance {
 rows, err := h.DB.Query(
 "SELECT sdk_no, port FROM sdk_instances WHERE status='RUNNING' AND sdk_no != ? AND port > 0 ORDER BY sdk_no",
 excludeSdkNo,
 )
 if err != nil {
 return nil
 }
 defer rows.Close()
 var insts []altInstance
 for rows.Next() {
 var inst altInstance
 if err := rows.Scan(&inst.sdkNo, &inst.port); err != nil {
 continue
 }
 insts = append(insts, inst)
 }
 return insts
}

func (h *Handler) smartDeviceInfo(sn string) (json.RawMessage, error) {
 log := func(msg string) {
 if h.Logger != nil {
 h.Logger.Log("smartroute", msg)
 }
 }

 sdkNo, port, err := h.resolveDevice(sn)
 if err != nil {
 return nil, err
 }

 var lastBusyData json.RawMessage

 data, err := h.Proxy.DeviceInfo(port, sn)
 if err != nil {
 log(sn + " → sdk-" + strconv.Itoa(sdkNo) + " (assigned, port:" + strconv.Itoa(port) + ") ERROR: " + err.Error())
 return nil, err
 }
 if !services.IsBusyResponse(data) {
 log(sn + " → sdk-" + strconv.Itoa(sdkNo) + " (assigned, port:" + strconv.Itoa(port) + ") OK")
 return data, nil
 }

 lastBusyData = data
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " busy → restarting in background → trying alternates")

 restartDone := make(chan error, 1)
 go func() {
 restartDone <- h.SdkMgr.Restart(sdkNo)
 }()

 for _, alt := range h.queryAlternatePorts(sdkNo) {
 altData, altErr := h.Proxy.DeviceInfo(alt.port, sn)
 if altErr != nil {
 log(sn + " → sdk-" + strconv.Itoa(alt.sdkNo) + " (alt, port:" + strconv.Itoa(alt.port) + ") ERROR: " + altErr.Error())
 continue
 }
 if !services.IsBusyResponse(altData) {
 log(sn + " → sdk-" + strconv.Itoa(alt.sdkNo) + " (fallback, sdk-" + strconv.Itoa(sdkNo) + " busy, port:" + strconv.Itoa(alt.port) + ") OK")
 return altData, nil
 }
 log(sn + " → sdk-" + strconv.Itoa(alt.sdkNo) + " (alt, port:" + strconv.Itoa(alt.port) + ") also busy")
 lastBusyData = altData
 }

 log(sn + " all alternates busy, waiting for sdk-" + strconv.Itoa(sdkNo) + " restart...")
 select {
 case restartErr := <-restartDone:
 if restartErr != nil {
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " restart FAILED: " + restartErr.Error())
 return lastBusyData, services.ErrFServiceBusy
 }
 case <-time.After(60 * time.Second):
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " restart TIMEOUT after 60s")
 return lastBusyData, services.ErrFServiceBusy
 }

 log(sn + " waiting for sdk-" + strconv.Itoa(sdkNo) + " port:" + strconv.Itoa(port) + " ready...")
 waitErr := services.WaitUntilReady(port, 15*time.Second)
 if waitErr != nil {
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " not ready after restart (port:" + strconv.Itoa(port) + "): " + waitErr.Error())
 }

 data, err = h.Proxy.DeviceInfo(port, sn)
 if err == nil && !services.IsBusyResponse(data) {
 log(sn + " → sdk-" + strconv.Itoa(sdkNo) + " (after restart, port:" + strconv.Itoa(port) + ") OK")
 return data, nil
 }

 if err != nil {
 lastBusyData = nil
 } else {
 lastBusyData = data
 }
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " STILL busy after restart → hard recreating...")

 log(sn + " hard recreating sdk-" + strconv.Itoa(sdkNo) + " (same id, port:" + strconv.Itoa(port) + ")...")
 if delErr := h.SdkMgr.Delete(sdkNo); delErr != nil {
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " delete FAILED: " + delErr.Error())
 return lastBusyData, services.ErrFServiceBusy
 }
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " deleted OK")

 if _, createErr := h.SdkMgr.Create(sdkNo, port); createErr != nil {
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " create FAILED: " + createErr.Error())
 return lastBusyData, services.ErrFServiceBusy
 }
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " created OK")

 if startErr := h.SdkMgr.Start(sdkNo); startErr != nil {
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " start FAILED: " + startErr.Error())
 return lastBusyData, services.ErrFServiceBusy
 }
 log(sn + " sdk-" + strconv.Itoa(sdkNo) + " started OK")

 h.DB.Exec("UPDATE devices SET sdk_no=?, updated_at=datetime('now') WHERE sn=?", sdkNo, sn)
 log(sn + " reassigned to sdk-" + strconv.Itoa(sdkNo) + ", final attempt")

 data, err = h.Proxy.DeviceInfo(port, sn)
 if err != nil {
 log(sn + " → sdk-" + strconv.Itoa(sdkNo) + " (after recreate) ERROR: " + err.Error())
 return lastBusyData, services.ErrFServiceBusy
 }
 if services.IsBusyResponse(data) {
 log(sn + " → sdk-" + strconv.Itoa(sdkNo) + " (after recreate) STILL BUSY — giving up")
 return data, services.ErrFServiceBusy
 }
 log(sn + " → sdk-" + strconv.Itoa(sdkNo) + " (after recreate) OK")
 return data, nil
}

func (h *Handler) HandleDeviceSetTime(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }
 data, err := h.Queue.Enqueue(sn, "dev/settime", nil)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleDeviceInit(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }
 data, err := h.Queue.Enqueue(sn, "dev/init", nil)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleDeviceDelAdmin(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }
 data, err := h.Queue.Enqueue(sn, "dev/deladmin", nil)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleLogDel(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }
 data, err := h.Queue.Enqueue(sn, "log/del", nil)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleSyncReload(w http.ResponseWriter, r *http.Request) {
	if err := h.Sync.ReloadFromRoot(); err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	go func() {
		for _, sdkNo := range h.SdkMgr.ListRunningSdkNos() {
			h.SdkMgr.Restart(sdkNo)
		}
	}()
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
}

func (h *Handler) HandleSyncStatus(w http.ResponseWriter, r *http.Request) {
 report := h.Watchdog.GetHealthReport()
 h.writeJSON(w, http.StatusOK, report)
}

func (h *Handler) HandleJobs(w http.ResponseWriter, r *http.Request) {
 rows, err := h.DB.Query(
 "SELECT id, sdk_no, sn, action, status, COALESCE(request,''), COALESCE(response,''), retry_count, created_at FROM jobs ORDER BY id DESC LIMIT 100",
 )
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 defer rows.Close()

 type job struct {
 ID int `json:"id"`
 SdkNo int `json:"sdk_no"`
 SN string `json:"sn"`
 Action string `json:"action"`
 Status string `json:"status"`
 Request string `json:"request"`
 Response string `json:"response"`
 RetryCount int `json:"retry_count"`
 CreatedAt string `json:"created_at"`
 }

 var jobs []job
 for rows.Next() {
 var j job
 if err := rows.Scan(&j.ID, &j.SdkNo, &j.SN, &j.Action, &j.Status, &j.Request, &j.Response, &j.RetryCount, &j.CreatedAt); err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 jobs = append(jobs, j)
 }
 if jobs == nil {
 jobs = []job{}
 }
 h.writeJSON(w, http.StatusOK, jobs)
}

func (h *Handler) writeRawJSON(w http.ResponseWriter, status int, data json.RawMessage) {
 w.Header().Set("Content-Type", "application/json")
 w.WriteHeader(status)
 w.Write(data)
}

func (h *Handler) HandleLogStream(w http.ResponseWriter, r *http.Request) {
 if h.Logger == nil {
 h.writeError(w, http.StatusServiceUnavailable, "logger not available")
 return
 }
 h.Logger.SSEHandler(w, r)
}

func (h *Handler) HandleLogs(w http.ResponseWriter, r *http.Request) {
 if h.Logger == nil {
 h.writeJSON(w, http.StatusOK, []interface{}{})
 return
 }
 entries := h.Logger.GetAll()
 if entries == nil {
 entries = make([]services.LogEntry, 0)
 }
	h.writeJSON(w, http.StatusOK, entries)
}

func (h *Handler) HandleTestDeviceInfo(w http.ResponseWriter, r *http.Request) {
	var req testDeviceInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.SN == "" || req.SdkNo <= 0 {
		h.writeError(w, http.StatusBadRequest, "sn and sdk_no required")
		return
	}

	var port int
	var status string
	err := h.DB.QueryRow("SELECT port, status FROM sdk_instances WHERE sdk_no = ?", req.SdkNo).Scan(&port, &status)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "instance not found")
		return
	}
	if status != "RUNNING" {
		h.writeError(w, http.StatusServiceUnavailable, "instance not running")
		return
	}
	if port <= 0 {
		h.writeError(w, http.StatusServiceUnavailable, "instance port invalid")
		return
	}

	data, err := h.Proxy.DeviceInfo(port, req.SN)
	if err != nil {
		h.writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	h.writeRawJSON(w, http.StatusOK, data)
}
