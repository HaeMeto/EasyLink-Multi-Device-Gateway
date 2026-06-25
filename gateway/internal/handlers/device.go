package handlers

import (
 "encoding/json"
 "fmt"
 "net/http"
 "strconv"

 "easylink/gateway/internal/models"
)

type deviceCreateRequest struct {
	Name         string            `json:"name"`
	SN           string            `json:"sn"`
	Activation   string            `json:"activation"`
	Password     string            `json:"password"`
	IP           string            `json:"ip"`
	EthernetPort string            `json:"ethernet_port"`
	SdkNo        int               `json:"sdk_no"`
	Online       int               `json:"online"`
	Enabled      int               `json:"enabled"`
	Extras       map[string]string `json:"extras"`
}

type deviceConfigRequest struct {
 Config map[string]string `json:"config"`
}

func (h *Handler) HandleListDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(
		"SELECT id, sdk_no, name, sn, activation, password, ip, ethernet_port, enabled, online, fail_count, COALESCE(last_offline,''), created_at, updated_at FROM devices ORDER BY id",
	)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		var d models.Device
		if err := rows.Scan(&d.ID, &d.SdkNo, &d.Name, &d.SN, &d.Activation, &d.Password, &d.IP, &d.EthernetPort, &d.Enabled, &d.Online, &d.FailCount, &d.LastOffline, &d.CreatedAt, &d.UpdatedAt); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		devices = append(devices, d)
	}
 if devices == nil {
 devices = []models.Device{}
 }
 h.writeJSON(w, http.StatusOK, devices)
}

func (h *Handler) HandleCreateDevice(w http.ResponseWriter, r *http.Request) {
 var req deviceCreateRequest
 if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
 h.writeError(w, http.StatusBadRequest, "invalid JSON")
 return
 }
 if req.SN == "" {
 h.writeError(w, http.StatusBadRequest, "sn is required")
 return
 }
 if req.Password == "" {
 req.Password = "0"
 }
	if req.EthernetPort == "" {
		req.EthernetPort = "5005"
	}
	res, err := h.DB.Exec(
		`INSERT INTO devices (sdk_no, name, sn, activation, password, ip, ethernet_port, online, enabled, fail_count, last_offline, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, 0, '', datetime('now'))`,
		req.SdkNo, req.Name, req.SN, req.Activation, req.Password, req.IP, req.EthernetPort, req.Enabled,
	)
 if err != nil {
 h.writeError(w, http.StatusConflict, err.Error())
 return
 }

 id, _ := res.LastInsertId()

	for k, v := range req.Extras {
		h.DB.Exec(
			`INSERT INTO device_config (device_id, config_key, config_value) VALUES (?, ?, ?)
			ON CONFLICT(device_id, config_key) DO UPDATE SET config_value=excluded.config_value`,
			id, k, v,
		)
	}

	h.Sync.SyncAfterDeviceChange()
	if req.SdkNo > 0 {
		go h.SdkMgr.Restart(req.SdkNo)
	}
	if h.Logger != nil {
		h.Logger.Log("device", fmt.Sprintf("Added %s", req.SN))
	}
	h.writeJSON(w, http.StatusCreated, map[string]interface{}{"id": id})
}

func (h *Handler) HandleGetDevice(w http.ResponseWriter, r *http.Request) {
 id, _ := strconv.Atoi(extractPathParam(r, 2))
 if id == 0 {
 h.writeError(w, http.StatusBadRequest, "missing id")
 return
 }

	var d models.Device
	err := h.DB.QueryRow(
		"SELECT id, sdk_no, name, sn, activation, password, ip, ethernet_port, enabled, online, fail_count, COALESCE(last_offline,''), created_at, updated_at FROM devices WHERE id = ?",
		id,
	).Scan(&d.ID, &d.SdkNo, &d.Name, &d.SN, &d.Activation, &d.Password, &d.IP, &d.EthernetPort, &d.Enabled, &d.Online, &d.FailCount, &d.LastOffline, &d.CreatedAt, &d.UpdatedAt)
 if err != nil {
 h.writeError(w, http.StatusNotFound, "device not found")
 return
 }
 h.writeJSON(w, http.StatusOK, d)
}

func (h *Handler) HandleUpdateDevice(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(extractPathParam(r, 2))
	if id == 0 {
		h.writeError(w, http.StatusBadRequest, "missing id")
		return
	}

	var req deviceCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	var oldSdkNo int
	h.DB.QueryRow("SELECT sdk_no FROM devices WHERE id = ?", id).Scan(&oldSdkNo)

	_, err := h.DB.Exec(
		`UPDATE devices SET sdk_no=?, name=?, sn=?, activation=?, password=?, ip=?, ethernet_port=?, online=?, enabled=?, fail_count=0, last_offline='', updated_at=datetime('now')
		WHERE id=?`,
		req.SdkNo, req.Name, req.SN, req.Activation, req.Password, req.IP, req.EthernetPort, req.Online, req.Enabled, id,
	)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	for k, v := range req.Extras {
		h.DB.Exec(
			`INSERT INTO device_config (device_id, config_key, config_value) VALUES (?, ?, ?)
			ON CONFLICT(device_id, config_key) DO UPDATE SET config_value=excluded.config_value`,
			id, k, v,
		)
	}

	h.Sync.SyncAfterDeviceChange()
	if oldSdkNo > 0 && oldSdkNo != req.SdkNo {
		go h.SdkMgr.Restart(oldSdkNo)
	}
	if req.SdkNo > 0 {
		go h.SdkMgr.Restart(req.SdkNo)
	}
	if h.Logger != nil {
		h.Logger.Log("device", fmt.Sprintf("Updated %s", req.SN))
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}

func (h *Handler) HandleToggleDevice(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(extractPathParam(r, 2))
	if id == 0 {
		h.writeError(w, http.StatusBadRequest, "missing id")
		return
	}

	var enabled int
	err := h.DB.QueryRow("SELECT enabled FROM devices WHERE id = ?", id).Scan(&enabled)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "device not found")
		return
	}

	newEnabled := 1 - enabled
	h.DB.Exec("UPDATE devices SET enabled = ? WHERE id = ?", newEnabled, id)

	if newEnabled == 1 {
		var sdkNo int
		h.DB.QueryRow("SELECT sdk_no FROM devices WHERE id = ?", id).Scan(&sdkNo)
		if sdkNo > 0 {
			go h.SdkMgr.Restart(sdkNo)
		}
	}

	if h.Logger != nil {
		h.Logger.Log("device", fmt.Sprintf("Toggled SN=%d enabled=%d", id, newEnabled))
	}
	h.writeJSON(w, http.StatusOK, map[string]int{"enabled": newEnabled})
}

func (h *Handler) HandleDeleteDevice(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(extractPathParam(r, 2))
	if id == 0 {
		h.writeError(w, http.StatusBadRequest, "missing id")
		return
	}

	var sn string
	var oldSdkNo int
	h.DB.QueryRow("SELECT sn, sdk_no FROM devices WHERE id = ?", id).Scan(&sn, &oldSdkNo)

	h.DB.Exec("DELETE FROM device_config WHERE device_id = ?", id)
	h.DB.Exec("DELETE FROM devices WHERE id = ?", id)

	h.Sync.SyncAfterDeviceChange()
	if oldSdkNo > 0 {
		go h.SdkMgr.Restart(oldSdkNo)
	}
	if h.Logger != nil {
		h.Logger.Log("device", fmt.Sprintf("Deleted %s", sn))
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) HandleGetDeviceConfig(w http.ResponseWriter, r *http.Request) {
 id, _ := strconv.Atoi(extractPathParam(r, 2))
 if id == 0 {
 h.writeError(w, http.StatusBadRequest, "missing id")
 return
 }

 rows, err := h.DB.Query(
 "SELECT id, device_id, config_key, config_value, created_at FROM device_config WHERE device_id = ?",
 id,
 )
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 defer rows.Close()

 var configs []models.DeviceConfig
 for rows.Next() {
 var c models.DeviceConfig
 if err := rows.Scan(&c.ID, &c.DeviceID, &c.ConfigKey, &c.ConfigValue, &c.CreatedAt); err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 configs = append(configs, c)
 }
 if configs == nil {
 configs = []models.DeviceConfig{}
 }
 h.writeJSON(w, http.StatusOK, configs)
}

func (h *Handler) HandleUpdateDeviceConfig(w http.ResponseWriter, r *http.Request) {
 id, _ := strconv.Atoi(extractPathParam(r, 2))
 if id == 0 {
 h.writeError(w, http.StatusBadRequest, "missing id")
 return
 }

 var req deviceConfigRequest
 if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
 h.writeError(w, http.StatusBadRequest, "invalid JSON")
 return
 }

	for k, v := range req.Config {
		h.DB.Exec(
			`INSERT INTO device_config (device_id, config_key, config_value) VALUES (?, ?, ?)
			ON CONFLICT(device_id, config_key) DO UPDATE SET config_value=excluded.config_value`,
			id, k, v,
		)
	}

	var sdkNo int
	h.DB.QueryRow("SELECT sdk_no FROM devices WHERE id = ?", id).Scan(&sdkNo)

	h.Sync.SyncAfterDeviceChange()
	if sdkNo > 0 {
		go h.SdkMgr.Restart(sdkNo)
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}

 func (h *Handler) HandleDeleteDeviceConfig(w http.ResponseWriter, r *http.Request) {
 id, _ := strconv.Atoi(extractPathParam(r, 2))
 key := extractPathParam(r, 4)
 if id == 0 || key == "" {
 h.writeError(w, http.StatusBadRequest, "missing id or key")
 return
 }

	h.DB.Exec("DELETE FROM device_config WHERE device_id = ? AND config_key = ?", id, key)

	var sdkNo int
	h.DB.QueryRow("SELECT sdk_no FROM devices WHERE id = ?", id).Scan(&sdkNo)

	h.Sync.SyncAfterDeviceChange()
	if sdkNo > 0 {
		go h.SdkMgr.Restart(sdkNo)
	}
	h.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
