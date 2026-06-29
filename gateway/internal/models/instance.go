package models

const (
	StatusStopped    = "STOPPED"
	StatusRunning    = "RUNNING"
	StatusError      = "ERROR"
	StatusBusy       = "BUSY"
	StatusBusyScanlog = "BUSY-SCANLOG"
	StatusBusyUser   = "BUSY-SCANUSER"
)

func IsBusyStatus(status string) bool {
	return status == StatusBusy || status == StatusBusyScanlog || status == StatusBusyUser
}

type SdkInstance struct {
 ID           int    `json:"id"`
 SdkNo        int    `json:"sdk_no"`
 Name         string `json:"name"`
 Path         string `json:"path"`
 Port         int    `json:"port"`
 PID          int    `json:"pid"`
 Status       string `json:"status"`
 RestartCount int    `json:"restart_count"`
 LastRestart  string `json:"last_restart"`
 CreatedAt    string `json:"created_at"`
}
