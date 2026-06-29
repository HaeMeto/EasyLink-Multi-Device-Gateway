package services

import (
 "encoding/json"
 "fmt"
 "os"
 "path/filepath"
 "sync"
 "time"
)

type FileLogger struct {
 logDir string
 ch chan LogEntry
 done chan struct{}
 currentDate string
 file *os.File
 mu sync.Mutex
}

func NewFileLogger(logDir string) (*FileLogger, error) {
 if err := os.MkdirAll(logDir, 0755); err != nil {
 return nil, fmt.Errorf("create log dir: %w", err)
 }

 fl := &FileLogger{
 logDir: logDir,
 ch: make(chan LogEntry, 1024),
 done: make(chan struct{}),
 }

 go fl.run()

 return fl, nil
}

func (fl *FileLogger) Write(entry LogEntry) {
 select {
 case fl.ch <- entry:
 default:
 }
}

func (fl *FileLogger) run() {
 for {
 select {
 case entry := <-fl.ch:
 fl.writeEntry(entry)
 case <-fl.done:
 if fl.file != nil {
 fl.file.Close()
 fl.file = nil
 }
 return
 }
 }
}

func (fl *FileLogger) writeEntry(entry LogEntry) {
 fl.mu.Lock()
 defer fl.mu.Unlock()

 today := time.Now().Format("2006-01-02")
 if today != fl.currentDate {
 if fl.file != nil {
 fl.file.Close()
 fl.file = nil
 }

 f, err := os.OpenFile(
 filepath.Join(fl.logDir, today+".jsonl"),
 os.O_APPEND|os.O_CREATE|os.O_WRONLY,
 0644,
 )
 if err != nil {
 return
 }
 fl.file = f
 fl.currentDate = today
 }

 data, err := json.Marshal(entry)
 if err != nil {
 return
 }
 data = append(data, '\n')

 fl.file.Write(data)
}

func (fl *FileLogger) ReadLogs(date string) ([]LogEntry, error) {
 path := filepath.Join(fl.logDir, date+".jsonl")
 f, err := os.Open(path)
 if err != nil {
 if os.IsNotExist(err) {
 return []LogEntry{}, nil
 }
 return nil, err
 }
 defer f.Close()

 var entries []LogEntry
 decoder := json.NewDecoder(f)
 for decoder.More() {
 var entry LogEntry
 if err := decoder.Decode(&entry); err != nil {
 continue
 }
 entries = append(entries, entry)
 }
 if entries == nil {
 entries = []LogEntry{}
 }
 return entries, nil
}

func (fl *FileLogger) Close() {
 close(fl.done)
}
