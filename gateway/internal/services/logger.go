package services

import (
 "encoding/json"
 "fmt"
 "net/http"
 "sync"
 "time"
)

type LogEntry struct {
 ID int `json:"id"`
 Timestamp string `json:"timestamp"`
 Type string `json:"type"`
 Message string `json:"message"`
}

type EventLogger struct {
 logs []LogEntry
 nextID int
 maxSize int
 mu sync.Mutex
 subscribers map[int]chan []byte
 subID int
 subMu sync.Mutex
 fileLogger *FileLogger
}

func NewEventLogger(maxSize int) *EventLogger {
 return &EventLogger{
 logs: make([]LogEntry, 0, maxSize),
 maxSize: maxSize,
 subscribers: make(map[int]chan []byte),
 }
}

func (el *EventLogger) SetFileLogger(fl *FileLogger) {
 el.fileLogger = fl
}

func (el *EventLogger) Log(typ, message string) {
 el.mu.Lock()
 entry := LogEntry{
 ID: el.nextID,
 Timestamp: time.Now().Format("2006-01-02 15:04:05"),
 Type: typ,
 Message: message,
 }
 el.nextID++
 if len(el.logs) >= el.maxSize {
 el.logs = el.logs[1:]
 }
 el.logs = append(el.logs, entry)
 el.mu.Unlock()

 el.broadcast(entry)

 if el.fileLogger != nil {
 el.fileLogger.Write(entry)
 }
}

func (el *EventLogger) GetAll() []LogEntry {
 el.mu.Lock()
 defer el.mu.Unlock()
 result := make([]LogEntry, len(el.logs))
 copy(result, el.logs)
 return result
}

func (el *EventLogger) Subscribe() (int, chan []byte) {
 el.subMu.Lock()
 defer el.subMu.Unlock()
 el.subID++
 id := el.subID
 ch := make(chan []byte, 64)
 el.subscribers[id] = ch

 initial := el.GetAll()
 data, _ := json.Marshal(initial)
 ch <- data

 return id, ch
}

func (el *EventLogger) Unsubscribe(id int) {
 el.subMu.Lock()
 defer el.subMu.Unlock()
 if ch, ok := el.subscribers[id]; ok {
 close(ch)
 delete(el.subscribers, id)
 }
}

func (el *EventLogger) Close() {
 el.subMu.Lock()
 defer el.subMu.Unlock()
 for id, ch := range el.subscribers {
 close(ch)
 delete(el.subscribers, id)
 }
}

func (el *EventLogger) broadcast(entry LogEntry) {
 data, _ := json.Marshal(entry)
 el.subMu.Lock()
 defer el.subMu.Unlock()
 for _, ch := range el.subscribers {
 select {
 case ch <- data:
 default:
 }
 }
}

func (el *EventLogger) SSEHandler(w http.ResponseWriter, r *http.Request) {
 flusher, ok := w.(http.Flusher)
 if !ok {
 http.Error(w, "streaming not supported", http.StatusInternalServerError)
 return
 }

 w.Header().Set("Content-Type", "text/event-stream")
 w.Header().Set("Cache-Control", "no-cache")
 w.Header().Set("Connection", "keep-alive")

 id, ch := el.Subscribe()
 defer el.Unsubscribe(id)

 ctx := r.Context()
 for {
 select {
 case <-ctx.Done():
 return
 case data, ok := <-ch:
 if !ok {
 return
 }
 fmt.Fprintf(w, "data: %s\n\n", data)
 flusher.Flush()
 }
 }
}
