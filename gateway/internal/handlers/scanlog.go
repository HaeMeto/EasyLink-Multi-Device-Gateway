package handlers

import (
 "encoding/json"
 "fmt"
 "net/http"
 "net/url"
 "strconv"
 "time"
)

func (h *Handler) HandleScanlogNew(w http.ResponseWriter, r *http.Request) {
	sn := extractPathParam(r, 2)
	if sn == "" {
		h.writeError(w, http.StatusBadRequest, "missing sn")
		return
	}
	data, err := h.Queue.Enqueue(sn, "scanlog/new", url.Values{})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleScanlogSmartFetch(w http.ResponseWriter, r *http.Request) {
	sn := extractPathParam(r, 2)
	if sn == "" {
		h.writeError(w, http.StatusBadRequest, "missing sn")
		return
	}

	if h.AbsenDB != nil {
		var status string
		var lastCheck string
		err := h.AbsenDB.QueryRow("SELECT scanlog_status, COALESCE(last_scan_check,'') FROM device_info WHERE sn = ?", sn).Scan(&status, &lastCheck)
		needsFetch := true
		if err == nil && status != "stale" && lastCheck != "" {
			t, parseErr := time.Parse("2006-01-02 15:04:05", lastCheck)
			if parseErr == nil && time.Since(t) <= 30*time.Second {
				needsFetch = false
			}
		}
		if needsFetch {
			if _, syncErr := h.Queue.Enqueue(sn, "scanlog/sync-new", nil); syncErr != nil {
				h.Logger.Log("proxy", "smart-fetch sync-new failed: "+syncErr.Error())
			}
		}

		page := parseIntParam(r, "page")
		if page < 1 {
			page = 1
		}
		size := parseIntParam(r, "size")
		if size < 1 || size > 500 {
			size = 100
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
		var logs []row
		h.AbsenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&total)
		rows, qErr := h.AbsenDB.Query(
			"SELECT id, sn, scan_date, pin, verify_mode, io_mode, work_code, created_at FROM scanlog WHERE sn = ? ORDER BY scan_date DESC LIMIT ? OFFSET ?",
			sn, size, offset,
		)
		if qErr == nil {
			defer rows.Close()
			for rows.Next() {
				var r row
				if rows.Scan(&r.ID, &r.SN, &r.ScanDate, &r.PIN, &r.VerifyMode, &r.IOMode, &r.WorkCode, &r.CreatedAt) == nil {
					logs = append(logs, r)
				}
			}
		}
		if logs == nil {
			logs = []row{}
		}

		result, _ := json.Marshal(map[string]interface{}{
			"total": total,
			"page":  page,
			"size":  size,
			"data":  logs,
		})
		h.writeRawJSON(w, http.StatusOK, result)
		return
	}

	data, err := h.Queue.Enqueue(sn, "scanlog/new", url.Values{})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleScanlogAll(w http.ResponseWriter, r *http.Request) {
 sn := extractPathParam(r, 2)
 if sn == "" {
 h.writeError(w, http.StatusBadRequest, "missing sn")
 return
 }
 limit := parseIntParam(r, "limit")
 params := url.Values{}
 if limit > 0 {
 params.Set("limit", strconv.Itoa(limit))
 }
 data, err := h.Queue.Enqueue(sn, "scanlog/all", params)
 if err != nil {
 h.writeError(w, http.StatusInternalServerError, err.Error())
 return
 }
 if h.AbsenDB != nil {
 type scanlogRow struct {
 SN string `json:"SN"`
 ScanDate string `json:"ScanDate"`
 PIN string `json:"PIN"`
 VerifyMode int `json:"VerifyMode"`
 IOMode int `json:"IOMode"`
 WorkCode int `json:"WorkCode"`
 }
 var page struct {
 Result bool
 Data []scanlogRow `json:"Data"`
 }
 if json.Unmarshal(data, &page) == nil {
 saved := 0
 for _, e := range page.Data {
 var cnt int
 h.AbsenDB.QueryRow(
 "SELECT COUNT(*) FROM scanlog WHERE sn=? AND scan_date=? AND pin=? AND verify_mode=? AND io_mode=? AND work_code=?",
 e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
 ).Scan(&cnt)
 if cnt == 0 {
 h.AbsenDB.Exec(
 "INSERT INTO scanlog (sn, scan_date, pin, verify_mode, io_mode, work_code) VALUES (?, ?, ?, ?, ?, ?)",
 e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
 )
 saved++
 }
 }
 if saved > 0 {
 var newCount int
 h.AbsenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&newCount)
 h.AbsenDB.Exec("INSERT OR IGNORE INTO device_info (sn, scanlog_count, user_count) VALUES (?, 0, 0)", sn)
 h.AbsenDB.Exec("UPDATE device_info SET scanlog_count = ?, scanlog_status = 'idle', last_scan_sync = datetime('now'), updated_at = datetime('now') WHERE sn = ?", newCount, sn)
 if h.Logger != nil {
 h.Logger.Log("proxy", fmt.Sprintf("%s scanlog/all saved: %d records", sn, saved))
 }
 }
 }
 }
 h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleScanlogDelete(w http.ResponseWriter, r *http.Request) {
	sn := extractPathParam(r, 2)
	if sn == "" {
		h.writeError(w, http.StatusBadRequest, "missing sn")
		return
	}
	data, err := h.Queue.Enqueue(sn, "scanlog/del", url.Values{})
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeRawJSON(w, http.StatusOK, data)
}

func (h *Handler) HandleScanlogGPS(w http.ResponseWriter, r *http.Request) {
	sn := extractPathParam(r, 2)
	if sn == "" {
		h.writeError(w, http.StatusBadRequest, "missing sn")
		return
	}
	byDate := r.URL.Query().Get("by_date")
	params := url.Values{}
	if byDate != "" {
		params.Set("by_date", byDate)
	}
	data, err := h.Queue.Enqueue(sn, "scanlog/gps", params)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.writeRawJSON(w, http.StatusOK, data)
}
