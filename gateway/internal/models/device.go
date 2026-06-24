package models

type Device struct {
	ID           int    `json:"id"`
	SdkNo        int    `json:"sdk_no"`
	Name         string `json:"name"`
	SN           string `json:"sn"`
	Activation   string `json:"activation"`
	Password     string `json:"password"`
	IP           string `json:"ip"`
	EthernetPort string `json:"ethernet_port"`
	Enabled      int    `json:"enabled"`
	Online       int    `json:"online"`
	FailCount    int    `json:"fail_count"`
	LastOffline  string `json:"last_offline"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type DeviceConfig struct {
 ID          int    `json:"id"`
 DeviceID    int    `json:"device_id"`
 ConfigKey   string `json:"config_key"`
 ConfigValue string `json:"config_value"`
 CreatedAt   string `json:"created_at"`
}
