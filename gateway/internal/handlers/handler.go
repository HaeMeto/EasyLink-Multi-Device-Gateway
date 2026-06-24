package handlers

import (
 "encoding/json"
 "net/http"
 "strconv"
 "strings"

 "easylink/gateway/internal/database"
 "easylink/gateway/internal/services"
)

type Handler struct {
 DB *database.DB
 SdkMgr *services.SdkManager
 Sync *services.SyncService
 Queue *services.QueueManager
 Watchdog *services.Watchdog
 Logger *services.EventLogger
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
 data, err := h.Queue.Enqueue(sn, "dev/info", nil)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
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
