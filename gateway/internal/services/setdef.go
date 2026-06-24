package services

import (
	"fmt"
	"os"
)

func GenerateSetDef(port int) string {
	return fmt.Sprintf(
		"[setting]\nport=%d\nuse_timeout=-1\ntimeout=5000\nuse_auto_restart=0\nval_auto_restart=23:00\n",
		port,
	)
}

func WriteSetDef(path string, port int) error {
	content := GenerateSetDef(port)
	return os.WriteFile(path, []byte(content), 0644)
}
