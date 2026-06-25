package handlers

import (
 "encoding/json"
 "fmt"
 "net/http"
 "strconv"
)

func (h *Handler) HandleAbsenScanLogs(w http.ResponseWriter, r *http.Request) {
	sn := extractPathParam(r, 2)
	if sn == "" {
		h.writeError(w, http.StatusBadRequest, "missing sn")
		return
	}
	if h.AbsenDB == nil {
		h.writeError(w, http.StatusServiceUnavailable, "absen db not available")
		return
	}

	page := parseIntParam(r, "page")
	if page < 1 {
		page = 1
	}
	size := parseIntParam(r, "size")
	if size < 1 || size > 200 {
		size = 50
	}
	offset := (page - 1) * size

	type row struct {
		ID         int    `json:"id"`
		SN         string `json:"sn"`
		ScanDate   string `json:"scan_date"`
		PIN        string `json:"pin"`
		VerifyMode int    `json:"verify_mode"`
		IOMode     int    `json:"io_mode"`
		WorkCode   int    `json:"work_code"`
		CreatedAt  string `json:"created_at"`
	}

	var total int
	h.AbsenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&total)

	rows, err := h.AbsenDB.Query(
		"SELECT id, sn, scan_date, pin, verify_mode, io_mode, work_code, created_at FROM scanlog WHERE sn = ? ORDER BY scan_date DESC LIMIT ? OFFSET ?",
		sn, size, offset,
	)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var logs []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.SN, &r.ScanDate, &r.PIN, &r.VerifyMode, &r.IOMode, &r.WorkCode, &r.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, r)
	}
	if logs == nil {
		logs = []row{}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"total": total,
		"page":  page,
		"size":  size,
		"data":  logs,
	})
}

func (h *Handler) HandleAbsenDeviceInfo(w http.ResponseWriter, r *http.Request) {
	sn := extractPathParam(r, 2)
	if sn == "" {
		h.writeError(w, http.StatusBadRequest, "missing sn")
		return
	}
	if h.AbsenDB == nil {
		h.writeError(w, http.StatusServiceUnavailable, "absen db not available")
		return
	}

 var info struct {
 SN string `json:"sn"`
 ScanlogCount int `json:"scanlog_count"`
 UserCount int `json:"user_count"`
 ScanlogStatus string `json:"scanlog_status"`
 UserStatus string `json:"user_status"`
 LastScanSync string `json:"last_scan_sync"`
 LastScanCheck string `json:"last_scan_check"`
 LastUserSync string `json:"last_user_sync"`
 CreatedAt string `json:"created_at"`
 UpdatedAt string `json:"updated_at"`
 }

 err := h.AbsenDB.QueryRow(
 "SELECT sn, scanlog_count, user_count, scanlog_status, user_status, COALESCE(last_scan_sync,''), COALESCE(last_scan_check,''), COALESCE(last_user_sync,''), created_at, updated_at FROM device_info WHERE sn = ?",
 sn,
 ).Scan(&info.SN, &info.ScanlogCount, &info.UserCount, &info.ScanlogStatus, &info.UserStatus, &info.LastScanSync, &info.LastScanCheck, &info.LastUserSync, &info.CreatedAt, &info.UpdatedAt)
 if err != nil {
 h.writeJSON(w, http.StatusOK, map[string]interface{}{"sn": sn, "scanlog_count": 0, "user_count": 0, "scanlog_status": "idle", "user_status": "idle"})
		return
	}
	h.writeJSON(w, http.StatusOK, info)
}

func (h *Handler) HandleAbsenUsersList(w http.ResponseWriter, r *http.Request) {
	sn := extractPathParam(r, 2)
	if sn == "" {
		h.writeError(w, http.StatusBadRequest, "missing sn")
		return
	}
	if h.AbsenDB == nil {
		h.writeError(w, http.StatusServiceUnavailable, "absen db not available")
		return
	}

	page := parseIntParam(r, "page")
	if page < 1 {
		page = 1
	}
	size := parseIntParam(r, "size")
	if size < 1 || size > 200 {
		size = 50
	}
	offset := (page - 1) * size

	type row struct {
		ID        int    `json:"id"`
		SN        string `json:"sn"`
		PIN       string `json:"pin"`
		Name      string `json:"name"`
		RFID      string `json:"rfid"`
		Password  string `json:"password"`
		Privilege string `json:"privilege"`
		CreatedAt string `json:"created_at"`
	}

	var total int
	h.AbsenDB.QueryRow("SELECT COUNT(*) FROM \"user\" WHERE sn = ?", sn).Scan(&total)

	rows, err := h.AbsenDB.Query(
		"SELECT id, sn, pin, name, rfid, password, privilege, created_at FROM \"user\" WHERE sn = ? ORDER BY id LIMIT ? OFFSET ?",
		sn, size, offset,
	)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var users []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.SN, &r.PIN, &r.Name, &r.RFID, &r.Password, &r.Privilege, &r.CreatedAt); err != nil {
			continue
		}
		users = append(users, r)
	}
	if users == nil {
		users = []row{}
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"total": total,
		"page":  page,
		"size":  size,
		"data":  users,
	})
}

func (h *Handler) HandleAbsenSyncUsers(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }

 configLimit := 30
 var cfgVal string
 if err := h.DB.QueryRow("SELECT value FROM config WHERE key = 'user_sync_limit'").Scan(&cfgVal); err == nil {
 if n, e := strconv.Atoi(cfgVal); e == nil && n > 0 {
 configLimit = n
 }
 }

 var req struct {
 SdkNo int `json:"sdk_no"`
 Limit int `json:"limit"`
 }
 if r.Body != nil {
 json.NewDecoder(r.Body).Decode(&req)
 }

 limit := req.Limit
 if limit <= 0 {
 limit = configLimit
 }

 if h.AbsenDB != nil {
 var dbCount, localCount int
 h.AbsenDB.QueryRow("SELECT COALESCE(user_count, 0) FROM device_info WHERE sn = ?", sn).Scan(&dbCount)
 h.AbsenDB.QueryRow("SELECT COUNT(*) FROM \"user\" WHERE sn = ?", sn).Scan(&localCount)
 if dbCount > 0 && dbCount == localCount {
 h.writeJSON(w, http.StatusOK, map[string]interface{}{
 "status": "already_synced",
 "user_count": dbCount,
 })
 return
 }
 }

 if req.SdkNo > 0 {
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
 if h.AbsenDB == nil {
 h.writeError(w, http.StatusServiceUnavailable, "absen db not available")
 return
 }
 data, err := h.Proxy.SyncUsersFull(h.AbsenDB, port, sn, limit, h.Logger)
 if err != nil {
 h.writeError(w, http.StatusBadGateway, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
 return
 }

 params := map[string][]string{}
 if limit > 0 {
 params["limit"] = []string{strconv.Itoa(limit)}
 }
 data, err := h.Queue.Enqueue(sn, "user/sync-full", params)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleAbsenScanlogSync(w http.ResponseWriter, r *http.Request) {
	sn := extractPathParam(r, 2)
	if sn == "" {
		h.writeError(w, http.StatusBadRequest, "missing sn")
		return
	}

 var req struct {
  SdkNo        int `json:"sdk_no"`
  DeviceScanlog int `json:"device_scanlog"`
 }
 if r.Body != nil {
 json.NewDecoder(r.Body).Decode(&req)
 }

 if h.AbsenDB != nil {
 var dbCount, localCount int
 h.AbsenDB.QueryRow("SELECT COALESCE(scanlog_count, 0) FROM device_info WHERE sn = ?", sn).Scan(&dbCount)
 h.AbsenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&localCount)
 if dbCount > 0 && dbCount == localCount {
 if req.DeviceScanlog > 0 && req.DeviceScanlog > localCount {
 if h.Logger != nil {
 h.Logger.Log("proxy", fmt.Sprintf("force sync bypass guard: device=%d local=%d", req.DeviceScanlog, localCount))
 }
 } else {
 h.writeJSON(w, http.StatusOK, map[string]interface{}{
 "status": "already_synced",
 "count": dbCount,
 })
 return
 }
 }
 }

 if req.SdkNo > 0 {
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
		if h.AbsenDB == nil {
			h.writeError(w, http.StatusServiceUnavailable, "absen db not available")
			return
		}
 data, err := h.Proxy.SyncScanlog(h.AbsenDB, port, sn, h.Logger)
		if err != nil {
			h.writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		h.writeRawJSON(w, http.StatusOK, data)
		return
	}

	data, err := h.Queue.Enqueue(sn, "scanlog/sync", nil)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleAbsenUserTemplates(w http.ResponseWriter, r *http.Request) {
	sn := extractPathParam(r, 2)
	pin := extractPathParam(r, 4)
	if sn == "" || pin == "" {
		h.writeError(w, http.StatusBadRequest, "missing sn or pin")
		return
	}
	if h.AbsenDB == nil {
		h.writeError(w, http.StatusServiceUnavailable, "absen db not available")
		return
	}

	var userID int
	err := h.AbsenDB.QueryRow("SELECT id FROM \"user\" WHERE sn = ? AND pin = ?", sn, pin).Scan(&userID)
	if err != nil {
		h.writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	rows, err := h.AbsenDB.Query(
		"SELECT finger_idx, alg_ver, template FROM template WHERE user_id = ?",
		userID,
	)
	if err != nil {
		h.writeJSON(w, http.StatusOK, []interface{}{})
		return
	}
	defer rows.Close()

	type templateRow struct {
		FingerIdx string `json:"finger_idx"`
		AlgVer    string `json:"alg_ver"`
		Template  string `json:"template"`
	}

	var templates []templateRow
	for rows.Next() {
		var t templateRow
		if rows.Scan(&t.FingerIdx, &t.AlgVer, &t.Template) == nil {
			templates = append(templates, t)
		}
	}
	if templates == nil {
		templates = []templateRow{}
	}
	h.writeJSON(w, http.StatusOK, templates)
}

func (h *Handler) HandleAbsenCompare(w http.ResponseWriter, r *http.Request) {
	sn := extractPathParam(r, 2)
	if sn == "" {
		h.writeError(w, http.StatusBadRequest, "missing sn")
		return
	}
	if h.AbsenDB == nil {
		h.writeError(w, http.StatusServiceUnavailable, "absen db not available")
		return
	}

 var localScanlog, localUsers int
 var lastSync string
 h.AbsenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&localScanlog)
 err := h.AbsenDB.QueryRow(
 "SELECT user_count, COALESCE(last_scan_sync,'') FROM device_info WHERE sn = ?",
 sn,
 ).Scan(&localUsers, &lastSync)
 if err != nil {
 localUsers = 0
 lastSync = ""
 }

 sdkNo, _ := strconv.Atoi(r.URL.Query().Get("sdk_no"))
 skipDevice := r.URL.Query().Get("skip_device") == "1"

 var infoData json.RawMessage
 deviceScanlog := 0
 deviceUsers := 0

 if skipDevice {
 deviceScanlog = -1
 deviceUsers = -1
 } else if sdkNo > 0 {
		var port int
		var status string
		err := h.DB.QueryRow("SELECT port, status FROM sdk_instances WHERE sdk_no = ?", sdkNo).Scan(&port, &status)
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
		data, err := h.Proxy.DeviceInfo(port, sn)
		if err != nil {
			h.writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		infoData = data
	} else {
		var err error
		infoData, err = h.Queue.Enqueue(sn, "dev/info", nil)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
 }
 if !skipDevice {
 deviceScanlog = 0
 deviceUsers = 0
 var devInfo struct {
 DEVINFO struct {
 AllPresensi string `json:"All Presensi"`
 User string `json:"User"`
 } `json:"DEVINFO"`
 }
 if json.Unmarshal(infoData, &devInfo) == nil {
 if n, e := strconv.Atoi(devInfo.DEVINFO.AllPresensi); e == nil {
 deviceScanlog = n
 }
 if n, e := strconv.Atoi(devInfo.DEVINFO.User); e == nil {
 deviceUsers = n
 }
 }
 }

 h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"scanlog": map[string]interface{}{
			"local":  localScanlog,
			"device": deviceScanlog,
			"synced": localScanlog == deviceScanlog && deviceScanlog > 0,
		},
		"users": map[string]interface{}{
			"local":  localUsers,
			"device": deviceUsers,
			"synced": localUsers == deviceUsers && deviceUsers > 0,
		},
		"last_sync": lastSync,
	})
}
