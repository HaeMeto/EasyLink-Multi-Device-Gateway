package database

import "fmt"

const currentVersion = 9

func (db *DB) Migrate() error {
 _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
 version INTEGER PRIMARY KEY,
 applied_at TEXT DEFAULT (datetime('now'))
 )`)
 if err != nil {
 return fmt.Errorf("create schema_version: %w", err)
 }

 var ver int
 err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&ver)
 if err != nil {
 return fmt.Errorf("read schema_version: %w", err)
 }

 for v := ver + 1; v <= currentVersion; v++ {
 if err := db.runMigration(v); err != nil {
 return err
 }
 }

 return nil
}

func (db *DB) runMigration(version int) error {
 var err error
 switch version {
 case 1:
 err = db.migrateV1()
	case 2:
		err = db.migrateV2()
	case 3:
		err = db.migrateV3()
	case 4:
		err = db.migrateV4()
	case 5:
		err = db.migrateV5()
	case 6:
		err = db.migrateV6()
	case 7:
		err = db.migrateV7()
	case 8:
		err = db.migrateV8()
	case 9:
		err = db.migrateV9()
	default:
 return fmt.Errorf("unknown migration version %d", version)
 }
 if err != nil {
 return fmt.Errorf("migration v%d: %w", version, err)
 }

 _, err = db.Exec("INSERT INTO schema_version (version) VALUES (?)", version)
 return err
}

func (db *DB) migrateV1() error {
 ddl := `
CREATE TABLE IF NOT EXISTS sdk_instances (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 sdk_no INTEGER NOT NULL UNIQUE,
 name TEXT NOT NULL,
 path TEXT NOT NULL,
 port INTEGER NOT NULL,
 pid INTEGER DEFAULT 0,
 status TEXT DEFAULT 'STOPPED',
 restart_count INTEGER DEFAULT 0,
 last_restart TEXT,
 created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS devices (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 sdk_no INTEGER NOT NULL DEFAULT 0,
 name TEXT NOT NULL,
 sn TEXT NOT NULL UNIQUE,
 activation TEXT NOT NULL DEFAULT '',
 password TEXT DEFAULT '0',
 ip TEXT NOT NULL,
 ethernet_port TEXT DEFAULT '5005',
 enabled INTEGER DEFAULT 1,
 created_at TEXT DEFAULT (datetime('now')),
 updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS device_config (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 device_id INTEGER NOT NULL,
 config_key TEXT NOT NULL,
 config_value TEXT NOT NULL,
 created_at TEXT DEFAULT (datetime('now')),
 FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
 UNIQUE(device_id, config_key)
);

CREATE TABLE IF NOT EXISTS jobs (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 sdk_no INTEGER NOT NULL,
 sn TEXT NOT NULL,
 action TEXT NOT NULL,
 status TEXT DEFAULT 'PENDING',
 request TEXT,
 response TEXT,
 retry_count INTEGER DEFAULT 0,
 created_at TEXT DEFAULT (datetime('now'))
);
`
 _, err := db.Exec(ddl)
 return err
}

func (db *DB) migrateV2() error {
 _, err := db.Exec(`
 CREATE TABLE IF NOT EXISTS devices_v2 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 sdk_no INTEGER NOT NULL DEFAULT 0,
 name TEXT NOT NULL,
 sn TEXT NOT NULL UNIQUE,
 activation TEXT NOT NULL DEFAULT '',
 password TEXT DEFAULT '0',
 ip TEXT NOT NULL,
 ethernet_port TEXT DEFAULT '5005',
 enabled INTEGER DEFAULT 1,
 created_at TEXT DEFAULT (datetime('now')),
 updated_at TEXT DEFAULT (datetime('now'))
 );
 INSERT OR IGNORE INTO devices_v2 (id, sdk_no, name, sn, activation, password, ip, ethernet_port, enabled, created_at, updated_at)
 SELECT id, sdk_no, name, sn, activation, password, ip, ethernet_port, enabled, created_at, updated_at FROM devices;
 DROP TABLE IF EXISTS devices;
 ALTER TABLE devices_v2 RENAME TO devices;

 CREATE TABLE IF NOT EXISTS device_config_v2 (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 device_id INTEGER NOT NULL,
 config_key TEXT NOT NULL,
 config_value TEXT NOT NULL,
 created_at TEXT DEFAULT (datetime('now')),
 UNIQUE(device_id, config_key)
 );
 INSERT OR IGNORE INTO device_config_v2 (id, device_id, config_key, config_value, created_at)
 SELECT id, device_id, config_key, config_value, created_at FROM device_config;
 DROP TABLE IF EXISTS device_config;
 ALTER TABLE device_config_v2 RENAME TO device_config;
	`)
	return err
}

func (db *DB) migrateV3() error {
	_, err := db.Exec(`ALTER TABLE devices ADD COLUMN online INTEGER DEFAULT 1`)
	return err
}

func (db *DB) migrateV4() error {
	_, err := db.Exec(`ALTER TABLE devices ADD COLUMN fail_count INTEGER DEFAULT 0`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`ALTER TABLE devices ADD COLUMN last_offline TEXT`)
	return err
}

func (db *DB) migrateV5() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS config (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return err
	}
 _, err = db.Exec(`INSERT OR IGNORE INTO config (key, value) VALUES ('scanlog_sync_interval', '60')`)
 return err
}

func (db *DB) migrateV6() error {
	_, err := db.Exec(`INSERT OR IGNORE INTO config (key, value) VALUES ('user_sync_limit', '10')`)
	return err
}

func (db *DB) migrateV7() error {
	_, err := db.Exec(`INSERT OR IGNORE INTO config (key, value) VALUES ('user_sync_mitigation_wait_seconds', '60')`)
	return err
}

func (db *DB) migrateV8() error {
 _, err := db.Exec(`INSERT OR IGNORE INTO config (key, value) VALUES ('max_spawn_sdk', '10')`)
 return err
}

func (db *DB) migrateV9() error {
 _, err := db.Exec(`INSERT OR IGNORE INTO config (key, value) VALUES ('setdef_use_timeout', '-1')`)
 if err != nil {
 return err
 }
 _, err = db.Exec(`INSERT OR IGNORE INTO config (key, value) VALUES ('setdef_timeout', '5000')`)
 if err != nil {
 return err
 }
 _, err = db.Exec(`INSERT OR IGNORE INTO config (key, value) VALUES ('setdef_use_auto_restart', '0')`)
 if err != nil {
 return err
 }
 _, err = db.Exec(`INSERT OR IGNORE INTO config (key, value) VALUES ('setdef_val_auto_restart', '23:00')`)
 return err
}
