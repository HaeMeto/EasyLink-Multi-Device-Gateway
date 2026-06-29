package services

import (
	"fmt"
	"os"
	"strconv"

	"easylink/gateway/internal/database"
)

func ReadSetDefConfig(db *database.DB) (useTimeout string, timeout string, useAutoRestart string, valAutoRestart string) {
	useTimeout = "-1"
	timeout = "5000"
	useAutoRestart = "0"
	valAutoRestart = "23:00"

	rows, err := db.Query("SELECT key, value FROM config WHERE key LIKE 'setdef_%'")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if rows.Scan(&key, &value) != nil {
			continue
		}
		switch key {
		case "setdef_use_timeout":
			useTimeout = value
		case "setdef_timeout":
			timeout = value
		case "setdef_use_auto_restart":
			useAutoRestart = value
		case "setdef_val_auto_restart":
			valAutoRestart = value
		}
	}
	return
}

func GenerateSetDef(port int, useTimeout, timeout, useAutoRestart, valAutoRestart string) string {
	ut, _ := strconv.Atoi(useTimeout)
	uar, _ := strconv.Atoi(useAutoRestart)
	return fmt.Sprintf(
		"[setting]\nport=%d\nuse_timeout=%d\ntimeout=%s\nuse_auto_restart=%d\nval_auto_restart=%s\n",
		port, ut, timeout, uar, valAutoRestart,
	)
}

func GenerateSetDefFromDB(port int, db *database.DB) string {
	ut, t, uar, var_ := ReadSetDefConfig(db)
	return GenerateSetDef(port, ut, t, uar, var_)
}

func WriteSetDef(path string, port int, db *database.DB) error {
	content := GenerateSetDefFromDB(port, db)
	return os.WriteFile(path, []byte(content), 0644)
}
