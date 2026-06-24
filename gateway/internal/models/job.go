package models

const (
 JobPending = "PENDING"
 JobRunning = "RUNNING"
 JobDone    = "DONE"
 JobError   = "ERROR"
)

type Job struct {
 ID         int    `json:"id"`
 SdkNo      int    `json:"sdk_no"`
 SN         string `json:"sn"`
 Action     string `json:"action"`
 Status     string `json:"status"`
 Request    string `json:"request"`
 Response   string `json:"response"`
 RetryCount int    `json:"retry_count"`
 CreatedAt  string `json:"created_at"`
}
