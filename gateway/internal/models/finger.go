package models

import (
 "encoding/json"
 "strconv"
)

type ScanlogEntry struct {
	ID         int    `json:"id"`
	SN         string `json:"SN"`
	ScanDate   string `json:"ScanDate"`
	PIN        string `json:"PIN"`
	VerifyMode int    `json:"VerifyMode"`
	IOMode     int    `json:"IOMode"`
	WorkCode   int    `json:"WorkCode"`
	CreatedAt  string `json:"created_at"`
}

type ScanlogPagingResponse struct {
	Result    bool           `json:"Result"`
	IsSession bool           `json:"IsSession"`
	Data      []ScanlogEntry `json:"Data"`
}

type DeviceInfoResponse struct {
	Result  bool `json:"Result"`
	DEVINFO struct {
		AllPresensi string `json:"All Presensi"`
		NewPresensi string `json:"New Presensi"`
		User        string `json:"User"`
	} `json:"DEVINFO"`
}

func (d *DeviceInfoResponse) GetAllPresensi() int {
 n, _ := strconv.Atoi(d.DEVINFO.AllPresensi)
 return n
}

func (d *DeviceInfoResponse) GetNewPresensi() int {
 n, _ := strconv.Atoi(d.DEVINFO.NewPresensi)
 return n
}

func (d *DeviceInfoResponse) GetUser() int {
	n, _ := strconv.Atoi(d.DEVINFO.User)
	return n
}

type UserEntry struct {
	ID        int             `json:"id"`
	SN        string          `json:"SN"`
	PIN       string          `json:"PIN"`
	Name      string          `json:"Name"`
	RFID      string          `json:"RFID"`
	Password  string          `json:"Password"`
 Privilege int `json:"Privilege"`
	CreatedAt string          `json:"created_at"`
	Templates []TemplateEntry `json:"Template,omitempty"`
}

type UserPagingResponse struct {
	Result    bool        `json:"Result"`
	IsSession bool        `json:"IsSession"`
	Data      []UserEntry `json:"Data"`
}

type TemplateEntry struct {
 FingerIdx int `json:"idx"`
 Pin string `json:"pin"`
 AlgVer int `json:"alg_ver"`
 Template string `json:"template"`
}

func (t *TemplateEntry) UnmarshalJSON(data []byte) error {
	var raw struct {
		FingerIdx json.Number `json:"idx"`
		Pin       string      `json:"pin"`
		AlgVer    json.Number `json:"alg_ver"`
		Template  string      `json:"template"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	t.Pin = raw.Pin
	t.Template = raw.Template
	t.FingerIdx, _ = strconv.Atoi(raw.FingerIdx.String())
	t.AlgVer, _ = strconv.Atoi(raw.AlgVer.String())
	return nil
}

type AbsenDeviceInfo struct {
	SN             string `json:"sn"`
	ScanlogCount   int    `json:"scanlog_count"`
	UserCount      int    `json:"user_count"`
	ScanlogStatus  string `json:"scanlog_status"`
	LastScanSync   string `json:"last_scan_sync"`
	LastScanCheck  string `json:"last_scan_check"`
	LastUserSync   string `json:"last_user_sync"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

type ConfigEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}
