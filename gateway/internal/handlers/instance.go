package handlers

import (
 "encoding/json"
 "net/http"
 "strconv"

 "easylink/gateway/internal/models"
)

type createInstanceRequest struct {
 SdkNo int `json:"sdk_no"`
 Port int `json:"port"`
}

func (h *Handler) HandleListInstances(w http.ResponseWriter, r *http.Request) {
 instances, err := h.SdkMgr.ListAll()
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 if instances == nil {
 instances = []models.SdkInstance{}
 }
 h.writeJSON(w, http.StatusOK, instances)
}

func (h *Handler) HandleCreateInstance(w http.ResponseWriter, r *http.Request) {
 var req createInstanceRequest
 if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
 h.writeError(w, http.StatusBadRequest, "invalid JSON")
 return
 }

 if req.SdkNo == 0 {
 var maxNo int
 h.DB.QueryRow("SELECT COALESCE(MAX(sdk_no), 0) FROM sdk_instances").Scan(&maxNo)
 req.SdkNo = maxNo + 1
 }
 if req.Port == 0 {
 var maxPort int
 h.DB.QueryRow("SELECT COALESCE(MAX(port), 7100) FROM sdk_instances").Scan(&maxPort)
 req.Port = maxPort + 1
 }

 inst, err := h.SdkMgr.Create(req.SdkNo, req.Port)
 if err != nil {
 h.writeError(w, http.StatusConflict, err.Error())
 return
 }
 h.writeJSON(w, http.StatusCreated, inst)
}

func (h *Handler) HandleDeleteInstance(w http.ResponseWriter, r *http.Request) {
 sdkNo, _ := strconv.Atoi(extractPathParam(r, 2))
 if sdkNo == 0 {
 h.writeError(w, http.StatusBadRequest, "missing sdk_no")
 return
 }
 if err := h.SdkMgr.Delete(sdkNo); err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) HandleStartInstance(w http.ResponseWriter, r *http.Request) {
 sdkNo, _ := strconv.Atoi(extractPathParam(r, 2))
 if sdkNo == 0 {
 h.writeError(w, http.StatusBadRequest, "missing sdk_no")
 return
 }
 if err := h.SdkMgr.Start(sdkNo); err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (h *Handler) HandleStopInstance(w http.ResponseWriter, r *http.Request) {
 sdkNo, _ := strconv.Atoi(extractPathParam(r, 2))
 if sdkNo == 0 {
 h.writeError(w, http.StatusBadRequest, "missing sdk_no")
 return
 }
 if err := h.SdkMgr.Stop(sdkNo); err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (h *Handler) HandleRestartInstance(w http.ResponseWriter, r *http.Request) {
 sdkNo, _ := strconv.Atoi(extractPathParam(r, 2))
 if sdkNo == 0 {
 h.writeError(w, http.StatusBadRequest, "missing sdk_no")
 return
 }
 if err := h.SdkMgr.Restart(sdkNo); err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}
