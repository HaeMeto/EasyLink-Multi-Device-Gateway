# EasyLink Gateway — memory.md (Project SSoT)

## Project Identity
- **Name**: EasyLink Gateway
- **Type**: Go backend + embedded SPA frontend
- **Target**: Windows (win32/amd64)
- **Go Version**: 1.25.1
- **Working Directory**: `D:\Project\Easylink`

---

## 1. Directory Structure

```
D:\Project\Easylink\
├── core/                  # Master SDK (READ-ONLY, Fingerspot)
├── Device.ini             # Root SSoT device config
├── easylink.db            # SQLite metadata (auto-created)
├── gateway.exe            # Built binary
├── build.ps1              # Build script
├── memory.md              # This file
├── plan.md                # Implementation plan & bug tracker
├── gateway/
│   ├── main.go            # Entry point, wiring, routes
│   ├── go.mod / go.sum    # Module easylink/gateway
│   ├── template/          # Embedded SDK copy (from core/)
│   ├── ui/
│   │   ├── index.html     # SPA entry (Alpine.js + Tailwind)
│   │   ├── js/app.js      # Alpine global store + API
│   │   └── css/app.css    # Minimal styles
│   └── internal/
│       ├── config/config.go
│       ├── database/
│       │   ├── database.go     # SQLite open/close (MaxOpenConns=1, DB mutex)
│       │   └── migrations.go  # Schema auto-migration (v2)
│       ├── models/             # instance.go, device.go, job.go
│       ├── services/
│       │   ├── sdk_manager.go       # Instance lifecycle + embed extraction
│       │   ├── device_ini.go        # Device.ini parser/generator
 │ │ ├── setdef.go # SetDef.fin generator
 │ │ ├── fservice.go # HTTP proxy to FService
 │ │ ├── sync.go # Device.ini ↔ DB bidirectional sync
 │ │ ├── queue.go # Per-device serial queue manager
 │ │ ├── watchdog.go # Health check + auto recovery
 │ │ ├── logger.go # Event logger (ring buffer + SSE)
 │ │ ├── process_windows.go # isProcessAlive + terminateProcess (Windows API)
│       │   └── sys_windows.go       # Windows SysProcAttr (HideWindow)
│       └── handlers/               # handler.go, instance.go, device.go, scanlog.go, user.go
├── instances/             # Runtime SDK instances (created by gateway)
└── logs/                  # Gateway runtime logs
```

---

## 2. Architecture

```
User/SPA → gateway:7100 → queue (per-device) → FService proxy → FService.exe → device
                                                      ↓ (127.0.0.1:{port})
                                                      deviceLookup(SN) → DB

Build: core/ → copy → template/ → embed → gateway.exe
Startup: gateway.exe → extract template → create instances → auto-migrate DB
→ parse Device.ini → upsert DB → regen root Device.ini → auto-start instances
Device.ini: root ← symlink (fallback copy) → instance directories
Watchdog (10s tick): check PID → port every tick, HTTP every 6th tick (~60s)
→ ldb lock check → recover if fail (tree kill + verify)
Event Log: ring buffer (500 entries) → SSE streaming → UI /logs page
```

---

## 3. API Routes (Go 1.22+ ServeMux)

| Method | Path | Handler |
|--------|------|---------|
| GET | /health | HandleHealth |
| GET | /api/instances | HandleListInstances |
| POST | /api/instances | HandleCreateInstance |
| POST | /api/instances/{id}/start | HandleStartInstance |
| POST | /api/instances/{id}/stop | HandleStopInstance |
| POST | /api/instances/{id}/restart | HandleRestartInstance |
| DELETE | /api/instances/{id} | HandleDeleteInstance |
| GET | /api/devices | HandleListDevices |
| POST | /api/devices | HandleCreateDevice |
| GET | /api/devices/{id} | HandleGetDevice |
| PUT | /api/devices/{id} | HandleUpdateDevice |
| POST | /api/devices/{id}/toggle | HandleToggleDevice |
| DELETE | /api/devices/{id} | HandleDeleteDevice |
| GET | /api/devices/{id}/config | HandleGetDeviceConfig |
| PUT | /api/devices/{id}/config | HandleUpdateDeviceConfig |
| DELETE | /api/devices/{id}/config/{key} | HandleDeleteDeviceConfig |
| GET | /api/devices/{sn}/info | HandleDeviceInfo |
| GET | /api/devices/{sn}/scan/new | HandleScanlogNew |
| GET | /api/devices/{sn}/scan/all | HandleScanlogAll |
| POST | /api/devices/{sn}/scan/delete | HandleScanlogDelete |
| GET | /api/devices/{sn}/scan/gps | HandleScanlogGPS |
| GET | /api/devices/{sn}/users | HandleUserAll |
| POST | /api/devices/{sn}/users | HandleUserSet |
| POST | /api/devices/{sn}/users/batch | HandleUserSetAll |
| DELETE | /api/devices/{sn}/users/{pin} | HandleUserDelete |
| DELETE | /api/devices/{sn}/users | HandleUserDeleteAll |
| POST | /api/devices/{sn}/time | HandleDeviceSetTime |
| POST | /api/devices/{sn}/init | HandleDeviceInit |
| POST | /api/devices/{sn}/deladmin | HandleDeviceDelAdmin |
| POST | /api/devices/{sn}/log/del | HandleLogDel |
| POST | /api/sync/reload | HandleSyncReload |
| GET | /api/sync/status | HandleSyncStatus |
| GET | /api/jobs | HandleJobs |
| GET | /api/logs | HandleLogs |
| GET | /api/logs/stream | HandleLogStream |
| GET | / | Serve SPA index.html |

---

## 4. Database Schema (SQLite, easylink.db)

- `schema_version` — migration tracking
- `sdk_instances` — instance registry (sdk_no, name, path, port, pid, status)
- `devices` — device registry (name, sn, activation, password, ip, ethernet_port, enabled, online, fail_count, last_offline, sdk_no=DEFAULT 0)
- `device_config` — extended config per device
- `jobs` — job history (sdk_no, sn, action, status, request, response)

WAL mode, foreign keys enabled, busy_timeout=5000, MaxOpenConns=1.

---

## 5. Running & Build

```
# Build (from project root):
.\build.ps1

# Run (from project root):
.\gateway.exe

# Smoke test:
curl http://localhost:7100/health
```

---

## 6. Watchdog Recovery Flow (Phase K + L + M + N — All Phases)

```
watchdog tick (10s):
 query WHERE status = 'RUNNING' → instances list
   ↓
 PHASE 1 — Instance Infra Check (per instance):
 checkInstanceInfra(sdkNo, port, pid, path):
   ↓
  [PID] isProcessAlive:
    FAIL + port OPEN  → findPIDByPort → DB update pid → return (PID STALE)
    FAIL + port CLOSED → instanceFailCount++ → [fail X/5] → >=5 → recoverInstance
   ↓
  [PORT] net.DialTimeout(3s):
    FAIL → instanceFailCount++ → [fail X/5] → >=5 → recoverInstance
   ↓
  [LDB] checkLDBLock:
    FAIL → instanceFailCount++ → [fail X/5] → >=5 → recoverInstance

 PHASE 2 — Device Health Check (per instance, every tick):
 checkDevicesForInstance(sdkNo, port):
   query devices WHERE sdk_no = ? AND enabled = 1
   per device:
     skip if online=0 AND last_offline < 30min ago
     retry if online=0 AND last_offline >= 30min → set online=1
     POST /dev/info with SN:
       OK  → reset fail_count, set online=1, clear last_offline
       BUSY (message_code=3) → skip (not counted)
       FAIL → fail_count++
            → >=5 → set online=0, last_offline=datetime('now'), log OFFLINE

--- RECOVERY PATH (instance only, not device) ---
 recoverInstance(sdkNo):
   DB: SET status='ERROR' → PauseWorker → Restart() [mutex] → ResumeWorker
   → delete(instanceFailCount, sdkNo)
```

**Key behavior (Phase 5-9):**
- Instance health (PID/Port/LDB) independent from device health (HTTP)
- Device offline after 5 consecutive HTTP failures — instance stays RUNNING
- Offline devices auto-retry after 30 minutes
- Disabled devices (enabled=0) permanently skipped
- Device health check runs every tick (no more 60s skip)
- Log prefixes: `[instance]` for infra, `[device]` for device health

---

## 7. Configuration (Environment Variables)

| Var | Default | Description |
|-----|---------|-------------|
| EASYLINK_CONFIG | config.json | Config file path |
| EASYLINK_CORE_PATH | .\core | SDK master directory |
| EASYLINK_INSTANCES_PATH | .\instances | Runtime instances directory |
| EASYLINK_DB_PATH | .\easylink.db | SQLite database path |
| EASYLINK_ROOT_DEVICE_INI_PATH | .\Device.ini | Root SSoT Device.ini |
| EASYLINK_GATEWAY_PORT | 7100 | Gateway HTTP listen port |
| EASYLINK_FSERVICE_START_PORT | 7110 | First FService port |
| EASYLINK_WATCHDOG_INTERVAL | 10s | Health check interval |

---

## 8. Key Design Decisions

| Decision | Reason |
|----------|--------|
| 1 SDK folder = 1 FService instance | Avoid FService queue conflicts |
| Full copy SDK per instance | FService reads files relative to exe path |
| Embedded template in binary | Single binary deployment |
| DB Mutex + MaxOpenConns(1) | Avoid SQLite connection pool deadlocks |
| Queue serial per device, parallel across | Prevent request overlap |
| Watchdog per 10s, not blocking | Fast failure detection |
| Device.ini symlink root → instance | Single source, fallback copy on failure |
| Tree kill + process/port verify on Stop | Ensure clean FService shutdown |
| HTTP health check every 6th tick (~60s) | Avoid FService error spam from sn=0 |
| HTTP check uses real SN from DB | Valid POST /dev/info, 5s client timeout |
| sdk_no=0 auto-reroute | Device without assignment → first RUNNING instance |
| Event logger ring buffer + SSE | Real-time UI logs without disk I/O |
| RestartAllRunning async goroutine | Non-blocking API response after device change |

---

## 9. Constraints (NEVER VIOLATE)

- NEVER modify core/ files
- NEVER reverse-engineer FService.exe
- NEVER use symlink/junction for SDK instances
- NEVER global queue — always per-device
- NEVER restart all instances at once
- NEVER open multiple SQLite connections without mutex

---

## 10. Sticky Flags

- !DB-MUTEX: All DB access through `db.Mutex().Lock()` to serialize queries
- !DB-MAXCONNS-1: MaxOpenConns must be 1 — Go pool deadlocks with modernc.org/sqlite + WAL
- !ROWS-CLOSE: Always pre-collect SQL rows into slices before calling nested queries — avoids connection pool deadlock
- !FK-NO-SDK: devices.sdk_no has no FK constraint (DEFAULT 0), devices can exist without instances
- !PATH-SEP: Windows path separator (backslash)
- !EMBED-ALL: Go 1.23+ `embed all:template` for recursive embed
- !PROCESS-CHECK: `isProcessAlive()` uses `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` → `GetExitCodeProcess(STILL_ACTIVE)` → `CloseHandle`
- !ROWS-CLOSE: Always pre-collect SQL rows into slices before calling nested queries

---

## 11. Known Bugs (Resolved)

### BUG-001 (FIXED): Status() killed FService
**File**: `internal/services/sdk_manager.go:232`
**Fix**: Replaced `proc.Signal(os.Kill)` with `isProcessAlive()` using `OpenProcess` API (process_windows.go)

### BUG-002 (FIXED): Watchdog PID check no-op
**File**: `internal/services/watchdog.go:125-141`
**Fix**: Replaced `proc.Signal(syscall.Signal(0))` with `isProcessAlive()`

### BUG-003 (FIXED): Instance ID wrong
**File**: `internal/services/sdk_manager.go:93`
**Fix**: Use `result.LastInsertId()` instead of `sdkNo` as instance ID

### ISS-001 (FIXED): Destructive migration v2
**File**: `internal/database/migrations.go:104-135`
**Fix**: Non-destructive copy-data → drop-old → rename-new pattern

### ISS-002 (FIXED): Empty Device.ini on create
**File**: `internal/services/sdk_manager.go:69-70`
**Fix**: Call `syncService.ExportToInstance()` after writing empty ini

### ISS-003 (FIXED): FullSync skipped stopped instances
**File**: `internal/services/sync.go:149`
**Fix**: Removed `WHERE status != 'STOPPED'` filter

### ISS-004 (FIXED): Drained queue never called
**File**: `internal/services/queue.go:146-154`
**Fix**: `PauseWorker` now calls `w.drainQueue()`

### SQLite Deadlock (FIXED)
**Fix**: DB-level mutex (`db.Mutex()`) + MaxOpenConns(1) + pre-collect rows before nested queries

### BUG-D1 (FIXED): FService tidak clean-kill
**File**: `internal/services/sdk_manager.go:185-211`, `watchdog.go:193-219`
**Fix**: Tree kill (`taskkill /F /T`) + retry loop verify process dead (10×500ms) + port released

### BUG-D2 (FIXED): Device.ini tidak sync ke instances
**File**: `internal/services/sdk_manager.go:72-79`, `sync.go`, `handlers/device.go`, `handlers/handler.go`
**Fix**: Symlink root Device.ini → instance (fallback copy). Deleted `ExportToInstance`, `exportToInstanceLocked`, `syncAllInstanceInisLocked`, `syncAllInstanceInis`. Added `RestartAllRunning()` async goroutine.

### BUG-D3 (FIXED): Watchdog spam POST /dev/info
**File**: `internal/services/watchdog.go:43,73-74,116`
**Fix**: `tickCount` field, HTTP check only every 6th tick (~60s). PID+port check tetap 10s.

### BUG-D4 (FIXED): Device sdk_no=0 tidak bisa digunakan
**File**: `gateway/main.go:105-112`
**Fix**: Fallback query `ORDER BY sdk_no LIMIT 1` untuk instance RUNNING pertama.

### FEAT-E1 (IMPLEMENTED): Console log page + event logger
**File**: `internal/services/logger.go` (new), `main.go`, `handler.go`, `sdk_manager.go`, `watchdog.go`, `queue.go`, `handlers/device.go`
**Fix**: `EventLogger` ring buffer (500 entries), SSE streaming, `GET /api/logs/stream` + `GET /api/logs`. Log types: instance, device, proxy, watchdog, system. CheckHTTP now uses valid SN from DB + 5s client timeout.

### FEAT-E2 (IMPLEMENTED): Console log UI
**File**: `ui/index.html`, `ui/js/app.js`
**Fix**: `/logs` route with dark terminal-style viewer, type-based color coding, auto-scroll, SSE real-time. `RestartAllRunning()` now async goroutine via EventLogger. GetHealthReport no longer calls checkHTTP (PID+port only).

### BUG-G1 (FIXED): Service spawn kembali setelah Stop (watchdog race)
**File**: `internal/services/sdk_manager.go:190-260`
**Fix**: DB update `status=STOPPED, pid=0` SEBELUM kill. Rollback DB jika kill gagal.

### BUG-G2 (FIXED): FService.exe zombie tidak mati setelah kill
**File**: `internal/services/sdk_manager.go:258-295`, `internal/services/process_windows.go:40-51`
**Fix**: Kill chain: `/F /T /PID` → `/F /IM FService.exe` → `TerminateProcess(pid)`. New helpers: `forceKill()`, `waitProcessDead()`, `terminateProcess()`.

### BUG-G3 (FIXED): Directory sdk-{N} tidak terhapus
**File**: `internal/services/sdk_manager.go:117-160`
**Fix**: Delete() propagate Stop() error, cek RemoveAll error + retry 3x500ms, DB only deleted after filesystem cleanup succeeds.

### BUG-G4 (FIXED): Device disabled saat instance dihapus
**File**: `internal/services/sdk_manager.go:155`
**Fix**: Hapus `enabled = 0`, hanya `sdk_no = 0` (unassign). Device tetap enabled, bisa di-reroute via BUG-D4.

### F-020 (FIXED): recoverInstance kill pattern tidak sinkron
**File**: `internal/services/sdk_manager.go:248-291`, `internal/services/watchdog.go:199-211`
**Fix**: forceKill + waitProcessDead jadi package-level function. Watchdog.recoverInstance() panggil forceKill() yang sama dengan Stop() — 3-level kill chain konsisten di kedua tempat. Hapus `os/exec` + `strconv` imports dari watchdog.

### F-021 (FIXED): RestartAllRunning race window
**File**: `internal/services/sdk_manager.go:350-367`
**Fix**: Re-query DB status setelah Stop(), skip Start jika status bukan STOPPED.

### F-022 (FIXED): taskkill /IM missing SysProcAttr
**File**: `internal/services/sdk_manager.go:256`
**Fix**: Tambah `cmd.SysProcAttr = sysProcAttr` pada command /IM FService.exe.

### BUG-G5 (FIXED): ImportFromRoot tidak re-enable device yang disabled
**File**: `internal/services/sync.go:125`, `gateway/main.go:70`
**Fix**: `importFromRootLocked()` UPDATE tambah `enabled=1`. One-time migration: `UPDATE devices SET enabled = 1 WHERE enabled = 0` setelah `FullSync()`. Device yang di-disable oleh old Delete code (pre-G4 fix) otomatis re-enabled saat gateway restart atau SyncReload.

### ISS-005 (FIXED): Shutdown hang + orphan FService
**File**: `internal/services/logger.go:86-93`, `gateway/main.go:207-229`, `internal/services/sdk_manager.go:374-390`
**Fix**: EventLogger.Close() tutup semua SSE subscriber channel. Shutdown sequence: cancel watchdog → Close() → stop instances parallel goroutine + WaitGroup → server.Shutdown(5s timeout). ListRunningSdkNos() untuk query instance RUNNING saat shutdown.

---

## 12. Open Gaps

- Instance auto-start on gateway restart: IMPLEMENTED
- Port availability check before instance create: NOT DONE
- Queue retry for pending jobs after recovery: NOT DONE
- Device soft-delete on root ini sync removal: NOT DONE
- API route `GET /api/instances/{id}/status`: NOT DONE
- Self-diagnostics on `/health` (DB ping, uptime): NOT DONE
- Test coverage: NOT STARTED

---

## 13. Known Issues (Minor — Third Audit, FIXED)

### F-014 (FIXED): Duplicate import logic in sync.go
`ImportFromRoot` at line 30 had inline logic duplicating `importFromRootLocked` at line 211.
**Fix**: `ImportFromRoot` delegates to `importFromRootLocked`.

### F-015 (FIXED): Dead deviceOpLoading in app.js
Property set on `self` but never declared in reactive state or used in HTML template.
**Fix**: Removed.

### F-016 (FIXED): Dead URL path parsing in instance.go
`HandleCreateInstance` parsed `parts[2]` from URL but route `POST /api/instances` has no wildcard segment.
**Fix**: Removed dead code + unused `strings` import.

### F-017 (FIXED): Navigation race condition in app.js
Async `getDeviceBySN` could set stale `deviceDetail` if user navigated away before fetch completed.
**Fix**: `_navToken` counter — incremented on each `navigate()`, checked before mutating state in async callback.

---

## 14. Status

All phases complete through Phase N (P1-P9).
Phase D-J + K + L + M: completed in previous sessions.
Phase N (P1-P4): Instance stability — targeted kill, 5-strike retry, mutex, online column.
Phase N (P5-P9): Device health separation — migration v4, fail_count/last_offline columns, watchdog split (instance vs device health), UI enable/disable toggle.
Bug fixed during P5-P9: SQLite deadlock in GetHealthReport and checkDevicesForInstance (nested QueryRow inside rows.Next with MaxOpenConns=1). Fixed by pre-collecting rows before nested queries.
