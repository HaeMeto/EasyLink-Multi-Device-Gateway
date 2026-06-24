package database

import (
 "database/sql"
 "fmt"
 "sync"
 "time"

 _ "modernc.org/sqlite"
)

type DB struct {
 *sql.DB
 mu sync.Mutex
}

func Open(dbPath string) (*DB, error) {
 dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
 raw, err := sql.Open("sqlite", dsn)
 if err != nil {
 return nil, fmt.Errorf("open sqlite: %w", err)
 }

 raw.SetMaxOpenConns(1)
 raw.SetMaxIdleConns(1)
 raw.SetConnMaxLifetime(0)

 if err := raw.Ping(); err != nil {
 return nil, fmt.Errorf("ping sqlite: %w", err)
 }

 if _, err := raw.Exec("PRAGMA journal_mode=WAL"); err != nil {
 return nil, fmt.Errorf("wal mode: %w", err)
 }
 if _, err := raw.Exec("PRAGMA foreign_keys=ON"); err != nil {
 return nil, fmt.Errorf("foreign keys: %w", err)
 }
 if _, err := raw.Exec("PRAGMA busy_timeout=5000"); err != nil {
 return nil, fmt.Errorf("busy timeout: %w", err)
 }

 db := &DB{DB: raw}
 return db, nil
}

func (db *DB) Mutex() *sync.Mutex {
 return &db.mu
}

func (db *DB) now() string {
 return time.Now().Format("2006-01-02 15:04:05")
}
