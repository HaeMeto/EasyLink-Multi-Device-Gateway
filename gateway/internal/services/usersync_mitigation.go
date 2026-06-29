package services

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"easylink/gateway/internal/database"
)

func MitigateUserSyncBusy(
	proxy *FServiceProxy,
	absenDB *database.DB,
	sdkMgr *SdkManager,
	db *database.DB,
	sn string,
	limit int,
	logger *EventLogger,
) (json.RawMessage, error) {
	waitSec := 60
	var cfgVal string
	if err := db.QueryRow("SELECT value FROM config WHERE key = 'user_sync_mitigation_wait_seconds'").Scan(&cfgVal); err == nil {
		if n, e := strconv.Atoi(cfgVal); e == nil && n > 0 {
			waitSec = n
		}
	}

	var maxSdkNo int
	db.QueryRow("SELECT COALESCE(MAX(sdk_no), 0) FROM sdk_instances").Scan(&maxSdkNo)
	var maxPort int
	db.QueryRow("SELECT COALESCE(MAX(port), 7109) FROM sdk_instances").Scan(&maxPort)
	newSdkNo := maxSdkNo + 1
	newPort := maxPort + 1

	_, err := sdkMgr.Create(newSdkNo, newPort)
	if err != nil {
		return nil, fmt.Errorf("mitigation create sdk-%d: %w", newSdkNo, err)
	}

	defer sdkMgr.Delete(newSdkNo)

	if err := sdkMgr.Start(newSdkNo); err != nil {
		return nil, fmt.Errorf("mitigation start sdk-%d: %w", newSdkNo, err)
	}

	if err := WaitUntilReady(newPort, 15*time.Second); err != nil {
		return nil, fmt.Errorf("mitigation sdk-%d not ready: %w", newSdkNo, err)
	}

	if logger != nil {
		logger.Log("proxy", fmt.Sprintf("mitigation spawned sdk-%d port=%d, waiting %ds", newSdkNo, newPort, waitSec))
	}
	time.Sleep(time.Duration(waitSec) * time.Second)

	infoData, err := proxy.DeviceInfo(newPort, sn)
	if err != nil {
		return nil, fmt.Errorf("mitigation dev/info sdk-%d: %w", newSdkNo, err)
	}

	if IsBusyResponse(infoData) {
		return nil, fmt.Errorf("mitigation sdk-%d still busy after %ds wait", newSdkNo, waitSec)
	}

	if logger != nil {
		logger.Log("proxy", fmt.Sprintf("mitigation dev/info OK, retrying sync on sdk-%d", newSdkNo))
	}

 return proxy.SyncUsersFull(absenDB, newPort, sn, limit, newSdkNo, logger)
}
