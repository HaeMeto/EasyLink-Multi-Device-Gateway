package services

import (
 "fmt"

 "easylink/gateway/internal/database"
 "easylink/gateway/internal/models"
)

type SyncService struct {
 db *database.DB
 rootIniPath string
}

func NewSyncService(db *database.DB, rootIniPath string) *SyncService {
 return &SyncService{db: db, rootIniPath: rootIniPath}
}

func (s *SyncService) RootIniPath() string {
 return s.rootIniPath
}

func (s *SyncService) GetInstancePath(sdkNo int) (string, error) {
 var path string
 err := s.db.QueryRow("SELECT path FROM sdk_instances WHERE sdk_no = ?", sdkNo).Scan(&path)
 return path, err
}

func (s *SyncService) ImportFromRoot(rootIniPath string) error {
 mu := s.db.Mutex()
 mu.Lock()
 defer mu.Unlock()
 return s.importFromRootLocked(rootIniPath)
}

func (s *SyncService) upsertDeviceConfig(devID int, extras map[string]string) error {
 for k, v := range extras {
 _, err := s.db.Exec(
 `INSERT INTO device_config (device_id, config_key, config_value) VALUES (?, ?, ?)
 ON CONFLICT(device_id, config_key) DO UPDATE SET config_value=excluded.config_value`,
 devID, k, v,
 )
 if err != nil {
 return fmt.Errorf("upsert config %s for device %d: %w", k, devID, err)
 }
 }
 return nil
}

func (s *SyncService) ExportToRoot(rootIniPath string) error {
 mu := s.db.Mutex()
 mu.Lock()
 defer mu.Unlock()

 entries, err := s.buildEntries()
 if err != nil {
 return err
 }
 return WriteDeviceIni(rootIniPath, entries)
}

func (s *SyncService) FullSync() error {
 mu := s.db.Mutex()
 mu.Lock()
 defer mu.Unlock()

 if err := s.importFromRootLocked(s.rootIniPath); err != nil {
 return fmt.Errorf("import: %w", err)
 }

 if err := s.exportToRootLocked(s.rootIniPath); err != nil {
 return fmt.Errorf("export root: %w", err)
 }
 return nil
}

func (s *SyncService) SyncAfterDeviceChange() error {
 mu := s.db.Mutex()
 mu.Lock()
 defer mu.Unlock()

 if err := s.exportToRootLocked(s.rootIniPath); err != nil {
 return fmt.Errorf("export root: %w", err)
 }
 return nil
}

func (s *SyncService) ReloadFromRoot() error {
 mu := s.db.Mutex()
 mu.Lock()
 defer mu.Unlock()

 if err := s.importFromRootLocked(s.rootIniPath); err != nil {
 return fmt.Errorf("import root: %w", err)
 }
 return s.exportToRootLocked(s.rootIniPath)
}

func (s *SyncService) importFromRootLocked(rootIniPath string) error {
 entries, err := ParseDeviceIni(rootIniPath)
 if err != nil {
 return fmt.Errorf("parse root Device.ini: %w", err)
 }

 for _, entry := range entries {
 if entry.SN == "" {
 continue
 }

 var devID int
 err := s.db.QueryRow("SELECT id FROM devices WHERE sn = ?", entry.SN).Scan(&devID)
 if err != nil {
 res, err := s.db.Exec(
 `INSERT INTO devices (sdk_no, name, sn, activation, password, ip, ethernet_port, enabled, updated_at)
 VALUES (0, ?, ?, ?, ?, ?, ?, 1, datetime('now'))`,
 entry.Name, entry.SN, entry.Activation, entry.Password, entry.IP, entry.EthernetPort,
 )
 if err != nil {
 return fmt.Errorf("insert device %s: %w", entry.SN, err)
 }
 id, _ := res.LastInsertId()
 devID = int(id)
 } else {
  _, err := s.db.Exec(
  `UPDATE devices SET name=?, activation=?, password=?, ip=?, ethernet_port=?, updated_at=datetime('now')
  WHERE id=?`,
 entry.Name, entry.Activation, entry.Password, entry.IP, entry.EthernetPort, devID,
 )
 if err != nil {
 return fmt.Errorf("update device %s: %w", entry.SN, err)
 }
 }

 s.upsertDeviceConfig(devID, entry.Extras)
 }

 return nil
}

func (s *SyncService) exportToRootLocked(rootIniPath string) error {
 entries, err := s.buildEntries()
 if err != nil {
 return err
 }
 return WriteDeviceIni(rootIniPath, entries)
}

func (s *SyncService) buildEntries() ([]DeviceIniEntry, error) {
 rows, err := s.db.Query(
 `SELECT id, name, sn, activation, password, ip, ethernet_port
 FROM devices WHERE enabled = 1 ORDER BY id`,
 )
 if err != nil {
 return nil, fmt.Errorf("query devices: %w", err)
 }

 type devRow struct {
 id int
 d models.Device
 }
 var devs []devRow
 for rows.Next() {
 var dr devRow
 if err := rows.Scan(&dr.id, &dr.d.Name, &dr.d.SN, &dr.d.Activation, &dr.d.Password, &dr.d.IP, &dr.d.EthernetPort); err != nil {
 rows.Close()
 return nil, fmt.Errorf("scan device: %w", err)
 }
 devs = append(devs, dr)
 }
 rows.Close()

 var entries []DeviceIniEntry
 for _, dr := range devs {
 entry := DeviceIniEntry{
 Name: dr.d.Name,
 SN: dr.d.SN,
 Activation: dr.d.Activation,
 Password: dr.d.Password,
 IP: dr.d.IP,
 EthernetPort: dr.d.EthernetPort,
 Extras: make(map[string]string),
 }
 extras, err := s.loadConfig(dr.id)
 if err != nil {
 return nil, err
 }
 entry.Extras = extras
 entries = append(entries, entry)
 }
 return entries, nil
}

func (s *SyncService) loadConfig(deviceID int) (map[string]string, error) {
 rows, err := s.db.Query(
 "SELECT config_key, config_value FROM device_config WHERE device_id = ?", deviceID,
 )
 if err != nil {
 return nil, fmt.Errorf("query config for device %d: %w", deviceID, err)
 }
 defer rows.Close()

 extras := make(map[string]string)
 for rows.Next() {
 var k, v string
 if err := rows.Scan(&k, &v); err != nil {
 return nil, err
 }
 extras[k] = v
 }
 return extras, rows.Err()
}
