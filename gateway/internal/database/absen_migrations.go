package database

import "fmt"

const absenCurrentVersion = 3

func (db *DB) AbsenMigrate() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at TEXT DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("create absen schema_version: %w", err)
	}

	var ver int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&ver)
	if err != nil {
		return fmt.Errorf("read absen schema_version: %w", err)
	}

	for v := ver + 1; v <= absenCurrentVersion; v++ {
		if err := db.runAbsenMigration(v); err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) runAbsenMigration(version int) error {
	var err error
	switch version {
	case 1:
		err = db.absenV1()
 case 2:
 err = db.absenV2()
 case 3:
 err = db.absenV3()
	default:
		return fmt.Errorf("unknown absen migration version %d", version)
	}
	if err != nil {
		return fmt.Errorf("absen migration v%d: %w", version, err)
	}

	_, err = db.Exec("INSERT INTO schema_version (version) VALUES (?)", version)
	return err
}

func (db *DB) absenV1() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS device_info (
		sn               TEXT PRIMARY KEY,
		scanlog_count    INTEGER DEFAULT 0,
		user_count       INTEGER DEFAULT 0,
		scanlog_status   TEXT DEFAULT 'idle',
		last_scan_sync   TEXT,
		last_scan_check  TEXT,
		last_user_sync   TEXT,
		created_at       TEXT DEFAULT (datetime('now')),
		updated_at       TEXT DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS scanlog (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		sn          TEXT NOT NULL,
		scan_date   TEXT NOT NULL,
		pin         TEXT NOT NULL,
		verify_mode TEXT DEFAULT '',
		io_mode     TEXT DEFAULT '',
		work_code   TEXT DEFAULT '',
		created_at  TEXT DEFAULT (datetime('now')),
		UNIQUE(sn, scan_date, pin)
	);

	CREATE TABLE IF NOT EXISTS "user" (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		sn          TEXT NOT NULL,
		pin         TEXT NOT NULL,
		name        TEXT DEFAULT '',
		rfid        TEXT DEFAULT '',
		password    TEXT DEFAULT '',
		privilege   TEXT DEFAULT '',
		created_at  TEXT DEFAULT (datetime('now')),
		UNIQUE(sn, pin)
	);

	CREATE TABLE IF NOT EXISTS template (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id     INTEGER REFERENCES "user"(id) ON DELETE CASCADE,
		finger_idx  TEXT DEFAULT '',
		alg_ver     TEXT DEFAULT '',
		template    TEXT DEFAULT ''
	);
	`
 _, err := db.Exec(ddl)
 return err
}

func (db *DB) absenV2() error {
 _, err := db.Exec("ALTER TABLE device_info ADD COLUMN user_status TEXT DEFAULT 'idle'")
 return err
}

func (db *DB) absenV3() error {
 if _, err := db.Exec(`
  CREATE TABLE user_v3 (
   id INTEGER PRIMARY KEY AUTOINCREMENT,
   sn TEXT NOT NULL,
   pin TEXT NOT NULL,
   name TEXT DEFAULT '',
   rfid TEXT DEFAULT '',
   password TEXT DEFAULT '',
   privilege INTEGER DEFAULT 0,
   created_at TEXT DEFAULT (datetime('now')),
   UNIQUE(sn, pin)
  );
  INSERT INTO user_v3 SELECT id, sn, pin, name, rfid, password, CAST(privilege AS INTEGER), created_at FROM "user";
  DROP TABLE "user";
  ALTER TABLE user_v3 RENAME TO "user";
 `); err != nil {
  return fmt.Errorf("rebuild user: %w", err)
 }

 if _, err := db.Exec(`
  CREATE TABLE template_v3 (
   id INTEGER PRIMARY KEY AUTOINCREMENT,
   user_id INTEGER REFERENCES "user"(id) ON DELETE CASCADE,
   finger_idx INTEGER DEFAULT 0,
   alg_ver INTEGER DEFAULT 0,
   template TEXT DEFAULT ''
  );
  INSERT INTO template_v3 SELECT id, user_id, CAST(finger_idx AS INTEGER), CAST(alg_ver AS INTEGER), template FROM template;
  DROP TABLE template;
  ALTER TABLE template_v3 RENAME TO template;
 `); err != nil {
  return fmt.Errorf("rebuild template: %w", err)
 }

 if _, err := db.Exec(`
  CREATE TABLE scanlog_v3 (
   id INTEGER PRIMARY KEY AUTOINCREMENT,
   sn TEXT NOT NULL,
   scan_date TEXT NOT NULL,
   pin TEXT NOT NULL,
   verify_mode INTEGER DEFAULT 0,
   io_mode INTEGER DEFAULT 0,
   work_code INTEGER DEFAULT 0,
   created_at TEXT DEFAULT (datetime('now')),
   UNIQUE(sn, scan_date, pin)
  );
  INSERT INTO scanlog_v3 SELECT id, sn, scan_date, pin, CAST(verify_mode AS INTEGER), CAST(io_mode AS INTEGER), CAST(work_code AS INTEGER), created_at FROM scanlog;
  DROP TABLE scanlog;
  ALTER TABLE scanlog_v3 RENAME TO scanlog;
 `); err != nil {
  return fmt.Errorf("rebuild scanlog: %w", err)
 }

 return nil
}

func (db *DB) Repair() error {
	if _, err := db.Exec(`DELETE FROM scanlog WHERE scan_date = '' OR pin = ''`); err != nil {
		return fmt.Errorf("repair delete corrupt: %w", err)
	}

	if _, err := db.Exec(`UPDATE device_info SET scanlog_count = (
		SELECT COUNT(*) FROM scanlog WHERE scanlog.sn = device_info.sn
	), scanlog_status = 'idle'`); err != nil {
		return fmt.Errorf("repair reconcile counts: %w", err)
	}

	return nil
}
