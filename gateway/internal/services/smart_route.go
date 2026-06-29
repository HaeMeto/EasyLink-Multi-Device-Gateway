package services

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"easylink/gateway/internal/database"
	"easylink/gateway/internal/models"
)

func ResolveSmartRoute(
	db *database.DB,
	sdkMgr *SdkManager,
	proxy *FServiceProxy,
	logger *EventLogger,
	sn string,
	assignedSdkNo int,
	assignedPort int,
	permanentSpawn bool,
) (sdkNo int, port int, cleanup func(), err error) {
	var status string
	if err := db.QueryRow("SELECT status FROM sdk_instances WHERE sdk_no = ?", assignedSdkNo).Scan(&status); err != nil {
		return 0, 0, nil, fmt.Errorf("smart-route %s: assigned sdk-%d not found", sn, assignedSdkNo)
	}

	if status == models.StatusRunning {
		if logger != nil {
			logger.Log("proxy", fmt.Sprintf("smart-route %s: using assigned sdk-%d (RUNNING)", sn, assignedSdkNo))
		}
		return assignedSdkNo, assignedPort, nil, nil
	}

	rows, err := db.Query("SELECT sdk_no, port FROM sdk_instances WHERE status = ? AND sdk_no != ?", models.StatusRunning, assignedSdkNo)
	if err == nil {
		defer rows.Close()
		type instRow struct {
			sdkNo int
			port  int
		}
		var alternates []instRow
		for rows.Next() {
			var r instRow
			if rows.Scan(&r.sdkNo, &r.port) == nil {
				alternates = append(alternates, r)
			}
		}
		if len(alternates) > 0 {
			pick := alternates[rand.Intn(len(alternates))]
			if logger != nil {
				logger.Log("proxy", fmt.Sprintf("smart-route %s: alternate sdk-%d (assigned sdk-%d %s)", sn, pick.sdkNo, assignedSdkNo, status))
			}
			return pick.sdkNo, pick.port, nil, nil
		}
	}

	rows2, err := db.Query("SELECT sdk_no, port FROM sdk_instances WHERE status = ?", models.StatusBusy)
	if err == nil {
		defer rows2.Close()
		var busyInsts []struct {
			sdkNo int
			port  int
		}
		for rows2.Next() {
			var r struct {
				sdkNo int
				port  int
			}
			if rows2.Scan(&r.sdkNo, &r.port) == nil {
				busyInsts = append(busyInsts, r)
			}
		}
		if len(busyInsts) > 0 {
			first := busyInsts[0]
			if logger != nil {
				logger.Log("proxy", fmt.Sprintf("smart-route %s: restarting sdk-%d (generic BUSY)", sn, first.sdkNo))
			}
			sdkNo, port, err = restartAndPoll(sdkMgr, proxy, logger, first.sdkNo, first.port)
			if err != nil {
				return 0, 0, nil, fmt.Errorf("smart-route %s: restart failed: %w", sn, err)
			}
			return sdkNo, port, nil, nil
		}
	}

	var maxSpawn int
	var cfgVal string
	if db.QueryRow("SELECT value FROM config WHERE key = 'max_spawn_sdk'").Scan(&cfgVal) == nil {
		if n, e := strconv.Atoi(cfgVal); e == nil && n > 0 {
			maxSpawn = n
		}
	}
	if maxSpawn == 0 {
		maxSpawn = 10
	}

	var currentCount int
	db.QueryRow("SELECT COUNT(*) FROM sdk_instances").Scan(&currentCount)
	if currentCount >= maxSpawn {
		return 0, 0, nil, fmt.Errorf("smart-route %s: max spawn limit reached (%d/%d)", sn, currentCount, maxSpawn)
	}

	if logger != nil {
		logger.Log("proxy", fmt.Sprintf("smart-route %s: spawning new SDK (all BUSY)", sn))
	}
	return spawnSdk(sdkMgr, db, proxy, logger, sn, permanentSpawn)
}

func restartAndPoll(sdkMgr *SdkManager, proxy *FServiceProxy, logger *EventLogger, sdkNo int, port int) (int, int, error) {
	if logger != nil {
		logger.Log("instance", fmt.Sprintf("restarting sdk-%d (async)", sdkNo))
	}

	restartDone := make(chan error, 1)
	go func() {
		restartDone <- sdkMgr.Restart(sdkNo)
	}()

	maxAttempts := 15
	for i := 0; i < maxAttempts; i++ {
		select {
		case err := <-restartDone:
			if err != nil {
				return 0, 0, fmt.Errorf("restart sdk-%d failed: %w", sdkNo, err)
			}
			if err := WaitUntilReady(port, 3*time.Second); err != nil {
				return 0, 0, fmt.Errorf("restart sdk-%d done but not ready: %w", sdkNo, err)
			}
			if logger != nil {
				logger.Log("instance", fmt.Sprintf("restarted sdk-%d ready", sdkNo))
			}
			return sdkNo, port, nil
		case <-time.After(2 * time.Second):
		}
	}

	return 0, 0, fmt.Errorf("restart sdk-%d timeout after %ds", sdkNo, maxAttempts*2)
}

func spawnSdk(sdkMgr *SdkManager, db *database.DB, proxy *FServiceProxy, logger *EventLogger, sn string, permanent bool) (int, int, func(), error) {
	var maxSdkNo int
	db.QueryRow("SELECT COALESCE(MAX(sdk_no), 0) FROM sdk_instances").Scan(&maxSdkNo)
	var maxPort int
	db.QueryRow("SELECT COALESCE(MAX(port), 7109) FROM sdk_instances").Scan(&maxPort)
	newSdkNo := maxSdkNo + 1
	newPort := maxPort + 1

	if logger != nil {
		logger.Log("instance", fmt.Sprintf("spawning sdk-%d port=%d for %s", newSdkNo, newPort, sn))
	}

	_, err := sdkMgr.Create(newSdkNo, newPort)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("spawn create sdk-%d: %w", newSdkNo, err)
	}

	if err := sdkMgr.Start(newSdkNo); err != nil {
		return 0, 0, nil, fmt.Errorf("spawn start sdk-%d: %w", newSdkNo, err)
	}

	if err := WaitUntilReady(newPort, 15*time.Second); err != nil {
		return 0, 0, nil, fmt.Errorf("spawn sdk-%d not ready: %w", newSdkNo, err)
	}

	if permanent {
		db.Exec("UPDATE devices SET sdk_no = ?, updated_at = datetime('now') WHERE sn = ?", newSdkNo, sn)
		if logger != nil {
			logger.Log("instance", fmt.Sprintf("smart-route %s: reassigned to sdk-%d (permanent)", sn, newSdkNo))
		}
		return newSdkNo, newPort, nil, nil
	}

	cleanup := func() {
		if logger != nil {
			logger.Log("instance", fmt.Sprintf("smart-route %s: deleting temporary sdk-%d", sn, newSdkNo))
		}
		sdkMgr.Delete(newSdkNo)
	}
	return newSdkNo, newPort, cleanup, nil
}
