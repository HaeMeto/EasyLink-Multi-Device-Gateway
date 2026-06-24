package services

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
)

var standardFields = map[string]bool{
	"sn":            true,
	"aktivasi":      true,
	"password":      true,
	"ip_address":    true,
	"ethernet_port": true,
}

var fieldOrder = []string{"sn", "aktivasi", "password", "ip_address", "ethernet_port"}

type DeviceIniEntry struct {
	Name         string
	SN           string
	Activation   string
	Password     string
	IP           string
	EthernetPort string
	Extras       map[string]string
}

func ParseDeviceIni(iniPath string) ([]DeviceIniEntry, error) {
	f, err := os.Open(iniPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", iniPath, err)
	}
	defer f.Close()

	var entries []DeviceIniEntry
	var current *DeviceIniEntry
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			if current != nil {
				entries = append(entries, *current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if current != nil {
				entries = append(entries, *current)
			}
			name := trimmed[1 : len(trimmed)-1]
			current = &DeviceIniEntry{
				Name:   name,
				Extras: make(map[string]string),
			}
			continue
		}

		if current == nil {
			continue
		}

		idx := strings.Index(trimmed, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		value := trimmed[idx+1:]

		switch key {
		case "sn":
			current.SN = value
		case "aktivasi":
			current.Activation = value
		case "password":
			current.Password = value
		case "ip_address":
			current.IP = value
		case "ethernet_port":
			current.EthernetPort = value
		default:
			current.Extras[key] = value
		}
	}

	if current != nil {
		entries = append(entries, *current)
	}

	return entries, scanner.Err()
}

func GenerateDeviceIni(entries []DeviceIniEntry) string {
	var sb strings.Builder
	for i, e := range entries {
		sb.WriteString(fmt.Sprintf("[%s]\n", e.Name))
		sb.WriteString(fmt.Sprintf("sn=%s\n", e.SN))
		sb.WriteString(fmt.Sprintf("aktivasi=%s\n", e.Activation))
		sb.WriteString(fmt.Sprintf("password=%s\n", e.Password))
		sb.WriteString(fmt.Sprintf("ip_address=%s\n", e.IP))
		sb.WriteString(fmt.Sprintf("ethernet_port=%s\n", e.EthernetPort))

		keys := make([]string, 0, len(e.Extras))
		for k := range e.Extras {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("%s=%s\n", k, e.Extras[k]))
		}

		if i < len(entries)-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func WriteDeviceIni(path string, entries []DeviceIniEntry) error {
	content := GenerateDeviceIni(entries)
	return os.WriteFile(path, []byte(content), 0644)
}
