package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"easylink/gateway/internal/database"
	"easylink/gateway/internal/models"
)

type FServiceProxy struct {
	client *http.Client
	logger *EventLogger
}

var tmplSdkTruncRE = regexp.MustCompile(`"template":"([^"]+)"`)

func NewFServiceProxy(logger *EventLogger) *FServiceProxy {
	return &FServiceProxy{
		logger: logger,
		client: &http.Client{
 Timeout: 900 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 4,
				IdleConnTimeout:     30 * time.Second,
				MaxConnsPerHost:     0,
			},
		},
	}
}

var ErrFServiceBusy = errors.New("device busy")

type busyResponse struct {
 Result bool `json:"Result"`
 MessageCode int `json:"message_code"`
}

func IsBusyResponse(data json.RawMessage) bool {
 var br busyResponse
 if err := json.Unmarshal(data, &br); err != nil {
 return false
 }
 return !br.Result && br.MessageCode == 3
}

func WaitUntilReady(port int, timeout time.Duration) error {
 deadline := time.Now().Add(timeout)
 for time.Now().Before(deadline) {
 conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
 if err == nil {
 conn.Close()
 req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/", port), nil)
 client := &http.Client{Timeout: 3 * time.Second}
 resp, err := client.Do(req)
 if err == nil {
 resp.Body.Close()
 if resp.StatusCode == http.StatusOK {
 return nil
 }
 }
 }
 time.Sleep(500 * time.Millisecond)
 }
 return fmt.Errorf("port %d not ready after %v", port, timeout)
}

func (p *FServiceProxy) SendRequest(port int, endpoint string, params url.Values) (json.RawMessage, error) {
 urlStr := fmt.Sprintf("http://127.0.0.1:%d/%s", port, strings.TrimPrefix(endpoint, "/"))
 body := params.Encode()

 if p.logger != nil {
  p.logger.Log("proxy", fmt.Sprintf("POST %s | %s", urlStr, body))
 }

 req, err := http.NewRequest("POST", urlStr, strings.NewReader(body))
 if err != nil {
 return nil, fmt.Errorf("create request: %w", err)
 }
 req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

 resp, err := p.client.Do(req)
 if err != nil {
 return nil, fmt.Errorf("request to FService port %d: %w", port, err)
 }
 defer resp.Body.Close()

 data, err := io.ReadAll(resp.Body)
 if err != nil {
 return nil, fmt.Errorf("read response: %w", err)
 }

 if resp.StatusCode != http.StatusOK {
 return nil, fmt.Errorf("FService returned %d: %s", resp.StatusCode, string(data))
 }

 return json.RawMessage(data), nil
}

func (p *FServiceProxy) DeviceInfo(port int, sn string) (json.RawMessage, error) {
 return p.SendRequest(port, "dev/info", url.Values{"sn": {sn}})
}

func (p *FServiceProxy) DeviceSetTime(port int, sn string) (json.RawMessage, error) {
 return p.SendRequest(port, "dev/settime", url.Values{"sn": {sn}})
}

func (p *FServiceProxy) DeviceInit(port int, sn string) (json.RawMessage, error) {
 return p.SendRequest(port, "dev/init", url.Values{"sn": {sn}})
}

func (p *FServiceProxy) DeviceDelAdmin(port int, sn string) (json.RawMessage, error) {
 return p.SendRequest(port, "dev/deladmin", url.Values{"sn": {sn}})
}

func (p *FServiceProxy) ScanlogNew(port int, sn string) (json.RawMessage, error) {
 return p.SendRequest(port, "scanlog/new", url.Values{"sn": {sn}})
}

func (p *FServiceProxy) ScanlogAll(port int, sn string, limit int) (json.RawMessage, error) {
 params := url.Values{"sn": {sn}}
 if limit > 0 {
 params.Set("limit", fmt.Sprintf("%d", limit))
 }
 return p.SendRequest(port, "scanlog/all/paging", params)
}

func (p *FServiceProxy) ScanlogDel(port int, sn string) (json.RawMessage, error) {
 return p.SendRequest(port, "scanlog/del", url.Values{"sn": {sn}})
}

func (p *FServiceProxy) ScanlogGPS(port int, sn string, byDate string) (json.RawMessage, error) {
 params := url.Values{"sn": {sn}}
 if byDate != "" {
 params.Set("by_date", byDate)
 }
 return p.SendRequest(port, "scanlog/gps", params)
}

func (p *FServiceProxy) UserAll(port int, sn string, limit int) (json.RawMessage, error) {
 params := url.Values{"sn": {sn}}
 if limit > 0 {
 params.Set("limit", fmt.Sprintf("%d", limit))
 }
 return p.SendRequest(port, "user/all/paging", params)
}

func (p *FServiceProxy) UserSet(port int, sn string, pin string, nama string, pwd string, rfid string, priv string, tmp string) (json.RawMessage, error) {
 params := url.Values{
 "sn": {sn},
 "pin": {pin},
 "nama": {nama},
 "pwd": {pwd},
 "rfid": {rfid},
 "priv": {priv},
 "tmp": {tmp},
 }
 return p.SendRequest(port, "user/set", params)
}

func (p *FServiceProxy) UserSetAll(port int, sn string, dataJSON string) (json.RawMessage, error) {
 return p.SendRequest(port, "user/set-all", url.Values{"sn": {sn}, "data": {dataJSON}})
}

func (p *FServiceProxy) UserDel(port int, sn string, pin string) (json.RawMessage, error) {
 return p.SendRequest(port, "user/del", url.Values{"sn": {sn}, "pin": {pin}})
}

func (p *FServiceProxy) UserDelAll(port int, sn string) (json.RawMessage, error) {
 return p.SendRequest(port, "user/delall", url.Values{"sn": {sn}})
}

func (p *FServiceProxy) LogDel(port int, sn string) (json.RawMessage, error) {
	return p.SendRequest(port, "log/del", url.Values{"sn": {sn}})
}

func (p *FServiceProxy) ScanlogAllFull(port int, sn string, limit int, logger *EventLogger) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 100
	}
	var all []models.ScanlogEntry
	pageNum := 0
	totalGot := 0
	for {
		data, err := p.ScanlogAll(port, sn, limit)
		if err != nil {
			return nil, err
		}
 var page models.ScanlogPagingResponse
 if err := json.Unmarshal(data, &page); err != nil {
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s scanlog/all parse failed: %v", sn, err))
 }
 return nil, fmt.Errorf("scanlog/all parse: %w", err)
 }
 pageNum++
		totalGot += len(page.Data)
		all = append(all, page.Data...)
		if !page.IsSession {
			if logger != nil {
				logger.Log("proxy", fmt.Sprintf("%s scanlog paging page=%d got=%d total=%d done", sn, pageNum, len(page.Data), totalGot))
			}
			break
		}
		if logger != nil {
			logger.Log("proxy", fmt.Sprintf("%s scanlog paging page=%d got=%d total=%d", sn, pageNum, len(page.Data), totalGot))
		}
	}
	result := models.ScanlogPagingResponse{Result: true, Data: all}
	return json.Marshal(result)
}

func (p *FServiceProxy) UserAllFull(port int, sn string, limit int, logger *EventLogger) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 10
	}
 var all []models.UserEntry
 pageNum := 0
 totalGot := 0
 for {
 data, err := p.UserAll(port, sn, limit)
 if err != nil {
 return nil, err
 }
 var page models.UserPagingResponse
 if err := json.Unmarshal(data, &page); err != nil {
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s user/all parse failed: %v", sn, err))
 }
 return nil, fmt.Errorf("user/all parse: %w", err)
 }
 pageNum++
 totalGot += len(page.Data)
 all = append(all, page.Data...)
 if !page.IsSession {
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s users paging page=%d got=%d total=%d done", sn, pageNum, len(page.Data), totalGot))
 }
 break
 }
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s users paging page=%d got=%d total=%d", sn, pageNum, len(page.Data), totalGot))
 }
 }
 result := models.UserPagingResponse{Result: true, Data: all}
 return json.Marshal(result)
}

func (p *FServiceProxy) SyncScanlog(absenDB *database.DB, port int, sn string, logger *EventLogger) (json.RawMessage, error) {
	infoData, err := p.DeviceInfo(port, sn)
	if err != nil {
		return nil, err
	}
 var devInfo models.DeviceInfoResponse
 if err := json.Unmarshal(infoData, &devInfo); err != nil {
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s dev/info parse failed: %v", sn, err))
 }
 return nil, fmt.Errorf("dev/info parse: %w", err)
 }
 allPresensi := devInfo.GetAllPresensi()
 newPresensi := devInfo.GetNewPresensi()

	var hasRow int
	absenDB.QueryRow("SELECT COUNT(*) FROM device_info WHERE sn = ?", sn).Scan(&hasRow)
	if hasRow == 0 {
		absenDB.Exec("INSERT OR IGNORE INTO device_info (sn, scanlog_count, user_count) VALUES (?, 0, 0)", sn)
	}

	absenDB.Exec("UPDATE device_info SET last_scan_check = datetime('now'), updated_at = datetime('now') WHERE sn = ?", sn)

	var localCount int
	absenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&localCount)

 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s sync start: device=%d local=%d gap=%d", sn, allPresensi, localCount, allPresensi-localCount))
 }

 if localCount == allPresensi && allPresensi > 0 {
 absenDB.Exec("UPDATE device_info SET scanlog_status = 'idle', updated_at = datetime('now') WHERE sn = ?", sn)
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s sync idle: device=%d local=%d", sn, allPresensi, localCount))
 }
 result, _ := json.Marshal(map[string]interface{}{"status": "idle", "count": localCount, "inserted": 0})
 return result, nil
 }

 absenDB.Exec("UPDATE device_info SET scanlog_status = 'syncing', updated_at = datetime('now') WHERE sn = ?", sn)

 if newPresensi > 0 {
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s sync new-presensi: %d, trying fast path", sn, newPresensi))
 }
 newData, newErr := p.ScanlogNew(port, sn)
 if newErr == nil {
 var newPage models.ScanlogPagingResponse
 if err := json.Unmarshal(newData, &newPage); err != nil {
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s scanlog/new parse failed in fast path: %v", sn, err))
 }
 } else {
 fastInserted := 0
 for _, e := range newPage.Data {
 var cnt int
 absenDB.QueryRow(
 "SELECT COUNT(*) FROM scanlog WHERE sn=? AND scan_date=? AND pin=? AND verify_mode=? AND io_mode=? AND work_code=?",
 e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
 ).Scan(&cnt)
 if cnt == 0 {
 absenDB.Exec(
 "INSERT INTO scanlog (sn, scan_date, pin, verify_mode, io_mode, work_code) VALUES (?, ?, ?, ?, ?, ?)",
 e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
 )
 fastInserted++
 }
 }
 var fastNewCount int
 absenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&fastNewCount)
 if fastNewCount == allPresensi {
 absenDB.Exec("UPDATE device_info SET scanlog_count = ?, scanlog_status = 'idle', last_scan_sync = datetime('now'), updated_at = datetime('now') WHERE sn = ?", fastNewCount, sn)
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s sync fast path done: +%d new", sn, fastInserted))
 }
 result, _ := json.Marshal(map[string]interface{}{"status": "synced", "count": fastNewCount, "inserted": fastInserted})
 return result, nil
 }
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s sync fast path partial: +%d new, still gap=%d, falling back to full", sn, fastInserted, allPresensi-fastNewCount))
 }
 }
 } else if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s sync fast path error: %v, falling back to full", sn, newErr))
 }
 } else {
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s sync new-presensi=0, gap=%d, using full pagination", sn, allPresensi-localCount))
 }
 }

 var data json.RawMessage
 data, err = p.ScanlogAllFull(port, sn, 100, logger)
	if err != nil {
		absenDB.Exec("UPDATE device_info SET scanlog_status = 'stale', updated_at = datetime('now') WHERE sn = ?", sn)
		if logger != nil {
			logger.Log("proxy", fmt.Sprintf("%s sync error: %v, device=%d local=%d", sn, err, allPresensi, localCount))
		}
		return nil, err
	}

 var page models.ScanlogPagingResponse
 if err := json.Unmarshal(data, &page); err != nil {
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s scanlog/all parse failed in fallback: %v", sn, err))
 }
 return nil, fmt.Errorf("scanlog/all parse: %w", err)
 }
 inserted := 0
	for _, e := range page.Data {
		var cnt int
		absenDB.QueryRow(
			"SELECT COUNT(*) FROM scanlog WHERE sn=? AND scan_date=? AND pin=? AND verify_mode=? AND io_mode=? AND work_code=?",
			e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
		).Scan(&cnt)
		if cnt == 0 {
			absenDB.Exec(
				"INSERT INTO scanlog (sn, scan_date, pin, verify_mode, io_mode, work_code) VALUES (?, ?, ?, ?, ?, ?)",
				e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
			)
			inserted++
		}
	}

	var newCount int
	absenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&newCount)
	absenDB.Exec("UPDATE device_info SET scanlog_count = ?, scanlog_status = 'idle', last_scan_sync = datetime('now'), updated_at = datetime('now') WHERE sn = ?", newCount, sn)

	if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s sync done: +%d new (%d→%d), device=%d", sn, newCount-localCount, localCount, newCount, allPresensi))
 }

 result, _ := json.Marshal(map[string]interface{}{"status": "synced", "count": newCount, "inserted": newCount - localCount})
 return result, nil
}

func (p *FServiceProxy) SyncScanlogNew(absenDB *database.DB, port int, sn string, logger *EventLogger) (json.RawMessage, error) {
 var hasRow int
 absenDB.QueryRow("SELECT COUNT(*) FROM device_info WHERE sn = ?", sn).Scan(&hasRow)
 if hasRow == 0 {
 absenDB.Exec("INSERT OR IGNORE INTO device_info (sn, scanlog_count, user_count) VALUES (?, 0, 0)", sn)
 }

 absenDB.Exec("UPDATE device_info SET last_scan_check = datetime('now'), scanlog_status = 'syncing', updated_at = datetime('now') WHERE sn = ?", sn)

 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s scanlog-new sync start", sn))
 }

 data, err := p.ScanlogNew(port, sn)
 if err != nil {
 absenDB.Exec("UPDATE device_info SET scanlog_status = 'stale', updated_at = datetime('now') WHERE sn = ?", sn)
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s scanlog-new sync failed: %v", sn, err))
 }
 return nil, err
 }

 var newData struct {
 Result bool `json:"Result"`
 Data []models.ScanlogEntry `json:"Data"`
 }
 if err := json.Unmarshal(data, &newData); err != nil {
 absenDB.Exec("UPDATE device_info SET scanlog_status = 'stale', updated_at = datetime('now') WHERE sn = ?", sn)
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s scanlog-new parse failed: %v", sn, err))
 }
 return nil, fmt.Errorf("scanlog/new parse: %w", err)
 }
 inserted := 0
 for _, e := range newData.Data {
 var cnt int
 absenDB.QueryRow(
 "SELECT COUNT(*) FROM scanlog WHERE sn=? AND scan_date=? AND pin=? AND verify_mode=? AND io_mode=? AND work_code=?",
 e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
 ).Scan(&cnt)
 if cnt == 0 {
 absenDB.Exec(
 "INSERT INTO scanlog (sn, scan_date, pin, verify_mode, io_mode, work_code) VALUES (?, ?, ?, ?, ?, ?)",
 e.SN, e.ScanDate, e.PIN, e.VerifyMode, e.IOMode, e.WorkCode,
 )
 inserted++
 }
 }

 var newCount int
 absenDB.QueryRow("SELECT COUNT(*) FROM scanlog WHERE sn = ?", sn).Scan(&newCount)
 absenDB.Exec("UPDATE device_info SET scanlog_count = ?, scanlog_status = 'idle', last_scan_sync = datetime('now'), updated_at = datetime('now') WHERE sn = ?", newCount, sn)

 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s scanlog-new sync done: +%d new (%d→%d)", sn, inserted, newCount-inserted, newCount))
 }

 result, _ := json.Marshal(map[string]interface{}{"status": "synced", "count": newCount, "inserted": inserted})
 return result, nil
}

func (p *FServiceProxy) SyncUsersFull(absenDB *database.DB, port int, sn string, limit int, sdkNo int, logger *EventLogger) (json.RawMessage, error) {
 if limit <= 0 {
 limit = 10
 }

 absenDB.Exec("INSERT OR IGNORE INTO device_info (sn, user_count) VALUES (?, 0)", sn)
 absenDB.Exec("UPDATE device_info SET user_status = 'syncing' WHERE sn = ?", sn)

 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s users sync start", sn))
 }

	// Data existing dipertahankan — no DELETE (AD-041)

	pageNum := 0
 totalGot := 0
 for {
		data, err := p.UserAll(port, sn, limit)
		if err != nil {
			absenDB.Exec("UPDATE device_info SET user_status = 'stale', updated_at = datetime('now') WHERE sn = ?", sn)
			if logger != nil {
				logger.Log("proxy", fmt.Sprintf("%s users sync failed: %v", sn, err))
			}
			return nil, err
		}

		if logger != nil {
			rawStr := tmplSdkTruncRE.ReplaceAllStringFunc(string(data), func(match string) string {
				val := match[12 : len(match)-1]
				if len(val) > 20 {
					return fmt.Sprintf(`"template":"%s..."`, val[:20])
				}
				return match
			})
 logger.Log(fmt.Sprintf("sdk-%d", sdkNo), fmt.Sprintf("%s user/all/paging response: %s", sn, rawStr))
		}

		var page models.UserPagingResponse
		if err := json.Unmarshal(data, &page); err != nil {
			absenDB.Exec("UPDATE device_info SET user_status = 'stale', updated_at = datetime('now') WHERE sn = ?", sn)
			if logger != nil {
				logger.Log("proxy", fmt.Sprintf("%s user/all parse failed: %v", sn, err))
			}
			return nil, fmt.Errorf("user/all parse: %w", err)
		}
	pageNum++

		if pageNum == 1 && IsBusyResponse(data) {
			absenDB.Exec("UPDATE device_info SET user_status = 'stale', updated_at = datetime('now') WHERE sn = ?", sn)
			if logger != nil {
				logger.Log("proxy", fmt.Sprintf("%s users sync busy (message_code:3)", sn))
			}
			return nil, ErrFServiceBusy
		}

		if pageNum == 1 && len(page.Data) == 0 {
			absenDB.Exec("UPDATE device_info SET user_status = 'stale', updated_at = datetime('now') WHERE sn = ?", sn)
			if logger != nil {
				logger.Log("proxy", fmt.Sprintf("%s users sync abort: FService returned 0 users, preserving existing data", sn))
			}
			return nil, fmt.Errorf("FService returned 0 users")
		}

		for _, e := range page.Data {
			if err := p.upsertUser(absenDB, sn, e); err != nil {
				if logger != nil {
					logger.Log("proxy", fmt.Sprintf("%s user upsert failed for pin=%s: %v", sn, e.PIN, err))
				}
			}
		}

 var count int
 absenDB.QueryRow("SELECT COUNT(*) FROM \"user\" WHERE sn = ?", sn).Scan(&count)
 absenDB.Exec("UPDATE device_info SET user_count = ? WHERE sn = ?", count, sn)

 totalGot += len(page.Data)
 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s users sync page=%d got=%d total=%d", sn, pageNum, len(page.Data), totalGot))
 }
 if !page.IsSession {
 break
 }
 }

 var count int
 absenDB.QueryRow("SELECT COUNT(*) FROM \"user\" WHERE sn = ?", sn).Scan(&count)

 var info models.AbsenDeviceInfo
 err := absenDB.QueryRow(
 "SELECT sn FROM device_info WHERE sn = ?", sn,
 ).Scan(&info.SN)
 if err != nil {
 absenDB.Exec("INSERT OR IGNORE INTO device_info (sn, user_count, user_status) VALUES (?, ?, 'idle')", sn, count)
 } else {
 absenDB.Exec("UPDATE device_info SET user_count = ?, user_status = 'idle', last_user_sync = datetime('now'), updated_at = datetime('now') WHERE sn = ?", count, sn)
 }

 if logger != nil {
 logger.Log("proxy", fmt.Sprintf("%s users sync done: %d users", sn, count))
 }

	result, _ := json.Marshal(map[string]interface{}{"status": "synced", "user_count": count})
	return result, nil
}

func (p *FServiceProxy) upsertUser(absenDB *database.DB, sn string, e models.UserEntry) error {
	var existingID int
	var existingName, existingRFID, existingPassword string
	var existingPrivilege int
	err := absenDB.QueryRow(
		`SELECT id, name, rfid, password, privilege FROM "user" WHERE sn = ? AND pin = ?`,
		sn, e.PIN,
	).Scan(&existingID, &existingName, &existingRFID, &existingPassword, &existingPrivilege)

	if errors.Is(err, sql.ErrNoRows) {
		res, execErr := absenDB.Exec(
			`INSERT INTO "user" (sn, pin, name, rfid, password, privilege) VALUES (?, ?, ?, ?, ?, ?)`,
			sn, e.PIN, e.Name, e.RFID, e.Password, e.Privilege,
		)
		if execErr != nil {
			return fmt.Errorf("insert user: %w", execErr)
		}
		userID, _ := res.LastInsertId()
		for _, t := range e.Templates {
			absenDB.Exec(
				"INSERT INTO template (user_id, finger_idx, alg_ver, template) VALUES (?, ?, ?, ?)",
				userID, t.FingerIdx, t.AlgVer, t.Template,
			)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("query user: %w", err)
	}

	if existingName != e.Name || existingRFID != e.RFID || existingPassword != e.Password || existingPrivilege != e.Privilege {
		absenDB.Exec(
			`UPDATE "user" SET name = ?, rfid = ?, password = ?, privilege = ? WHERE id = ?`,
			e.Name, e.RFID, e.Password, e.Privilege, existingID,
		)
	}

	type existingTemplate struct {
		fingerIdx int
		algVer    int
		template  string
	}
	var existingTemplates []existingTemplate
	rows, err := absenDB.Query("SELECT finger_idx, alg_ver, template FROM template WHERE user_id = ?", existingID)
	if err == nil {
		for rows.Next() {
			var et existingTemplate
			if scanErr := rows.Scan(&et.fingerIdx, &et.algVer, &et.template); scanErr == nil {
				existingTemplates = append(existingTemplates, et)
			}
		}
		rows.Close()
	}

	for _, t := range e.Templates {
		found := false
		for _, et := range existingTemplates {
			if et.fingerIdx == t.FingerIdx {
				found = true
				if et.algVer != t.AlgVer || et.template != t.Template {
					absenDB.Exec(
						"UPDATE template SET alg_ver = ?, template = ? WHERE user_id = ? AND finger_idx = ?",
						t.AlgVer, t.Template, existingID, t.FingerIdx,
					)
				}
				break
			}
		}
		if !found {
			absenDB.Exec(
				"INSERT INTO template (user_id, finger_idx, alg_ver, template) VALUES (?, ?, ?, ?)",
				existingID, t.FingerIdx, t.AlgVer, t.Template,
			)
		}
	}

	return nil
}