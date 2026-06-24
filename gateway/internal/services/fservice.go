package services

import (
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "strings"
 "time"
)

type FServiceProxy struct {
 client *http.Client
}

func NewFServiceProxy() *FServiceProxy {
 return &FServiceProxy{
 client: &http.Client{
 Timeout: 300 * time.Second,
 },
 }
}

func (p *FServiceProxy) SendRequest(port int, endpoint string, params url.Values) (json.RawMessage, error) {
 urlStr := fmt.Sprintf("http://127.0.0.1:%d/%s", port, strings.TrimPrefix(endpoint, "/"))
 body := params.Encode()

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
