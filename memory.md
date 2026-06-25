# EasyLink Gateway — memory.md (Project SSoT)

## Project Identity
- **Name**: EasyLink Gateway
- **Type**: Go backend + embedded SPA frontend
- **Target**: Windows (win32/amd64)
- **Go Version**: 1.25.1
- **Working Directory**: `D:\Project\Easylink`
- **Databases**: `easylink.db` (config/metadata), `absen.db` (scanlog/user data)

---

## 1. Directory Structure

```
D:\Project\Easylink\
├── core/                  # Master SDK (READ-ONLY, Fingerspot)
├── Device.ini             # Root SSoT device config
├── easylink.db            # SQLite metadata (auto-created)
├── absen.db               # SQLite scanlog/user data (auto-created)
├── gateway.exe            # Built binary
├── build.ps1              # Build script
├── memory.md              # This file
├── plan.md                # Implementation plan & bug tracker
├── gateway/
│   ├── main.go            # Entry point, wiring, routes
│   ├── go.mod / go.sum    # Module easylink/gateway
│ ├── template/ # Embedded SDK copy (from core/)
│ ├── templates/
│ │ ├── base.html # Layout shell (head, body, toast, confirm, sidebar, JS include)
│ │ ├── toast.html # Toast notification partial
│ │ ├── confirm.html # Confirm dialog partial
│ │ ├── sidebar.html # Sidebar nav partial
│ │ └── pages/ # Page templates (one per route)
│ │    ├── dashboard.html
│ │    ├── instances.html
│ │    ├── devices.html
│ │    ├── device-detail.html
│ │    ├── scanlog.html
│ │    ├── users.html
│ │    ├── test.html
│ │    ├── jobs.html
│ │    ├── logs.html
│ │    └── settings.html
│ ├── ui/
│ │ ├── js/app.js # Alpine global store + API
│ │ └── css/app.css # Minimal styles
│   └── internal/
│       ├── config/config.go
│       ├── database/
│       │   ├── database.go         # SQLite open/close (MaxOpenConns=1, DB mutex)
│ │ ├── migrations.go # easylink.db schema auto-migration (v6)
│       │   └── absen_migrations.go # absen.db schema auto-migration (v1) + Repair()
│       ├── models/
│       │   ├── instance.go        # instance.go
│       │   ├── device.go          # device.go
│       │   ├── job.go             # job.go
│       │   └── finger.go          # ScanlogEntry, UserEntry, TemplateEntry, AbsenDeviceInfo
│       ├── services/
│       │   ├── sdk_manager.go       # Instance lifecycle + embed extraction
│       │   ├── device_ini.go        # Device.ini parser/generator
│       │   ├── setdef.go            # SetDef.fin generator
│       │   ├── fservice.go          # HTTP proxy to FService + sync methods (ScanlogAllFull, SyncScanlog uses COUNT(*))
│       │   ├── sync.go              # Device.ini ↔ DB bidirectional sync
│       │   ├── queue.go             # Per-device serial queue manager
│       │   ├── watchdog.go          # Health check + auto recovery
│       │   ├── syncer.go            # Periodic auto-sync goroutine
│       │   ├── logger.go            # Event logger (ring buffer + SSE)
│       │   ├── process_windows.go   # isProcessAlive + terminateProcess (Windows API)
│       │   └── sys_windows.go       # Windows SysProcAttr (HideWindow)
│       └── handlers/
│           ├── handler.go    # Handler struct + health/logs/jobs/sync
│           ├── instance.go   # instance CRUD
│           ├── device.go     # device CRUD
│           ├── scanlog.go    # scanlog proxy + smart fetch
│           ├── user.go       # user proxy
│           ├── absen.go      # absen.db query + sync triggers
│           └── config.go     # GET/PUT config
├── instances/             # Runtime SDK instances (created by gateway)
└── logs/                  # Gateway runtime logs
```

---

## 2. Architecture

```
User/Browser → gateway:7100 → queue (per-device) → FService proxy → FService.exe → device
                                                        ↓ (127.0.0.1:{port})
                                                        deviceLookup(SN) → DB

Multi-Page (Go html/template): Dashboard, Instances, Devices, Scanlog, Users, Test, Jobs, Logs, Settings

Build: core/ → copy → template/ → embed → gateway.exe
Startup: gateway.exe → extract template → create instances → auto-migrate DB
→ migrasi absen.db → Repair() → parse Device.ini → upsert DB → regen root Device.ini → auto-start instances
Device.ini: root ← symlink (fallback copy) → instance directories
Watchdog (10s tick): check PID → port every tick, HTTP every tick
→ ldb lock check → recover if fail (tree kill + verify)
Event Log: ring buffer (500 entries) → SSE streaming → UI /logs page
Syncer (configurable interval, default 60s): iterate devices → compare FService counts → sync scanlog to absen.db
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
| GET | /api/devices/{sn}/scan/new | HandleScanlogNew (pure proxy) |
| GET | /api/devices/{sn}/scan/smart | HandleScanlogSmartFetch |
| GET | /api/devices/{sn}/scan/all | HandleScanlogAll |
| POST | /api/devices/{sn}/scan/delete | HandleScanlogDelete |
| GET | /api/devices/{sn}/scan/gps | HandleScanlogGPS |
| GET | /api/devices/{sn}/scan/logs | HandleAbsenScanLogs |
| GET | /api/devices/{sn}/scan/smart | HandleScanlogSmartFetch |
| POST | /api/devices/{sn}/scan/sync | HandleAbsenScanlogSync |
| GET | /api/devices/{sn}/users | HandleUserAll |
| POST | /api/devices/{sn}/users | HandleUserSet |
| POST | /api/devices/{sn}/users/batch | HandleUserSetAll |
| GET | /api/devices/{sn}/absen/compare | HandleAbsenCompare |
| POST | /api/devices/{sn}/users/sync | HandleAbsenSyncUsers |
| GET | /api/devices/{sn}/users/{pin}/templates | HandleAbsenUserTemplates |
| DELETE | /api/devices/{sn}/users/{pin} | HandleUserDelete |
| DELETE | /api/devices/{sn}/users | HandleUserDeleteAll |
| GET | /api/devices/{sn}/absen/info | HandleAbsenDeviceInfo |
| GET | /api/devices/{sn}/absen/users | HandleAbsenUsersList |
| POST | /api/devices/{sn}/time | HandleDeviceSetTime |
| POST | /api/devices/{sn}/init | HandleDeviceInit |
| POST | /api/devices/{sn}/deladmin | HandleDeviceDelAdmin |
| POST | /api/devices/{sn}/log/del | HandleLogDel |
| POST | /api/sync/reload | HandleSyncReload |
| GET | /api/sync/status | HandleSyncStatus |
| GET | /api/jobs | HandleJobs |
| GET | /api/logs | HandleLogs |
| GET | /api/logs/stream | HandleLogStream |
| GET | /api/config | HandleGetConfig |
| PUT | /api/config | HandlePutConfig |
| POST | /api/test/device-info | HandleTestDeviceInfo |

### Page Routes (Go html/template, non-API)

| Method | Pattern | Template | Description |
|---|---|---|---|
| GET | /{$} | dashboard | Instance health, stats, Sync Reload |
| GET | /instances | instances | CRUD instances table + create modal |
| GET | /devices | devices | CRUD devices table + edit modal |
| GET | /devices/{sn} | device-detail | Device info + operations panel |
| GET | /scanlog | scanlog | Device dropdown, paginated scanlog, sync |
| GET | /users | users | Device dropdown, paginated users, sync |
| GET | /test | test | Device/instance selectors, Device Info test |
| GET | /jobs | jobs | Job history log table |
| GET | /logs | logs | Real-time SSE log viewer |
| GET | /settings | settings | Scanlog sync interval config |

---

## 4. Database Schema (SQLite)

### easylink.db
- `schema_version` — migration tracking
- `sdk_instances` — instance registry (sdk_no, name, path, port, pid, status)
- `devices` — device registry (name, sn, activation, password, ip, ethernet_port, enabled, online, fail_count, last_offline, sdk_no=DEFAULT 0)
- `device_config` — extended config per device
- `jobs` — job history (sdk_no, sn, action, status, request, response)
- `config` — global config (key, value, updated_at), e.g. `scanlog_sync_interval`

### absen.db
- `device_info` — sync state per device (sn, scanlog_count, user_count, scanlog_status, last_scan_sync, last_scan_check, last_user_sync)
- `scanlog` — cached scanlog data (sn, scan_date, pin, verify_mode, io_mode, work_code), UNIQUE(sn, scan_date, pin). Columns are TEXT in DB; Go model uses `int` (SQLite dynamic typing).
- `"user"` — cached user data (sn, pin, name, rfid, password, privilege), UNIQUE(sn, pin)
- `template` — fingerprint templates (user_id FK, finger_idx, alg_ver, template)

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

## 5b. UI Pages (Multi-Page Routes, Go html/template)

| URL Path | Template | Description |
|---|---|---|
| / | dashboard | Instance health, stats, Sync Reload |
| /instances | instances | CRUD instances |
| /devices | devices | CRUD devices, enable/disable toggle |
| /devices/{sn} | device-detail | Device info + operations panel |
| /scanlog | scanlog | Device dropdown, paginated scanlog table, sync |
| /users | users | Device dropdown, paginated user table, sync |
| /test | test | Device selector, endpoint buttons, raw response |
| /jobs | jobs | Job history log |
| /logs | logs | Real-time SSE log viewer |
| /settings | settings | Scanlog sync interval config |

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
| EASYLINK_ABSEN_DB_PATH | .\absen.db | Absen SQLite database path |
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
| Parallel device sync (semaphore max 3) | Device sync tidak saling blokir; setiap goroutine punya context cancellation sendiri |
| Dual-lane queue (fast bypass / slow serial) | dev/info & dev/settime bypass worker channel; scanlog/user sync tetap serial per instance |
| Watchdog busy cooldown (60s) | Device yang return MessageCode=3 di-skip selama 60 detik; dua method publik IsDeviceInCooldown/MarkDeviceBusy |
| FServiceProxy connection pooling | http.Transport MaxIdleConnsPerHost=4, IdleConnTimeout=30s agar concurrent request tidak starvation |

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
- User sync from syncer (currently scanlog-only): NOT DONE
- Scanlog delete handling in absen.db: NOT DONE

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

### BUG-X1 (FIXED): Migration v2 destructive + scanlog_count mismatch
**File:** `internal/database/absen_migrations.go:99-136`, `services/fservice.go:203-217`
**Fix:** Rollback ke v1. Hapus v2 (DROP/recreate). Tambah `Repair()` idempotent: DELETE corrupt + UPDATE count + SET idle. `SyncScanlog` selalu `ScanlogAllFull` untuk disaster recovery.

### BUG-X2 (FIXED): Syncer auto full pagination spam
**File:** `internal/services/syncer.go:134`
**Fix:** Syncer auto hanya `ScanlogNew`. Full pagination (`ScanlogAllFull`) hanya via manual `POST /scan/sync`.

### BUG-R1 (FIXED): scanlog_count inkonsisten antar path sync
**File:** `internal/services/fservice.go:219,263`
**Fix:** `SyncScanlog` dan `SyncScanlogNew` sekarang pakai `SELECT COUNT(*)` dari absen.db, bukan `allPresensi` dari device. DeviceInfo call di `SyncScanlogNew` dihapus (unused).

### BUG-R2 (FIXED): Sync Now gagal tanpa feedback
**File:** `ui/js/app.js:189`
**Fix:** `catch (e) {}` diganti toast error: `'Sync failed: ' + e.message`.

### BUG-R3 (SUPERSEDED by S1-S3): Instance selector tidak difungsikan di scanlog page
**File:** `ui/index.html:245-253`, `ui/js/app.js:16`, `handlers/absen.go:192-241`
**Fix:** Selector dihapus di Phase R, direstore dengan wiring lengkap di Phase S — user pilih instance spesifik untuk Sync Now langsung (bypass queue). Default "Auto" tetap via queue.

---

## 14. Status

All phases complete through Phase F (D-W, X, R, S, C, F).
Phase D-J + K + L + M: completed in previous sessions.
Phase N (P1-P4): Instance stability — targeted kill, 5-strike retry, mutex, online column.
Phase N (P5-P9): Device health separation — migration v4, fail_count/last_offline columns, watchdog split.
Phase O (O1-O9): Scanlog absen sync system — absen.db, syncer goroutine, smart fetch, config table, UI sync settings.
Phase P (P1-P7): JSON deserialization fixes — DEVINFO nested struct, scanlog/user case mismatch, API compat preserved.
Phase T (T1-T4): UI overhaul — Scanlog page, Users page, Settings page, Device detail nav enhancement.
Phase U (U1-U4): Bug fixes (handler resolution, template expand) + Testing page.
Phase V (V1-V4): Direct testing endpoint (bypass queue) + sync comparison on scanlog/users pages.
Phase W (W1-W3): Compare undefined crash fix (nested init + x-if guards). W2 confirmed false alarm.
Phase X (X1-X3): Sync & Data Repair — rollback migration v2, Repair() idempotent, ScanlogAllFull always for manual sync.
Phase R (R1-R3): Audit & Polish — COUNT(*) unification in all sync paths, error toast on sync fail, remove unused instance selector.
Phase S (S1-S3): Instance Selector — restore selector + wire sdk_no to POST /scan/sync, backend branch direct/queue.
Phase C (C0-C3): Compare Button — memory hardening (sections 15-23), HandleAbsenCompare sdk_no param, Compare button UI, split fetchScanlogStatus + doCompare.
Phase F (F1): Scanlog Instance List — fix nambah fetchInstances() di navigate('/scanlog').

---

## 15. Stable Areas (PROVEN — DO NOT TOUCH)

- **Syncer**: auto sync only `ScanlogNew` via `doDeviceSync()`. Guard: `scanlog_status == 'syncing'` skip. `COUNT(*)` for accuracy. **MODIFIED — PERFORMANCE**: parallel goroutine-per-device dengan semaphore max 3, busy cooldown integration via `*Watchdog` reference.
- **Watchdog**: two-phase health check — instance infra (PID/Port/LDB) + device HTTP (per-device POST /dev/info with 5-strike retry). **MODIFIED — PERFORMANCE**: busy cooldown map (60s), exported `IsDeviceInCooldown`/`MarkDeviceBusy` untuk syncer.
- **Queue Manager**: per-device serial queue. Parallel across devices. Never global queue. **MODIFIED — PERFORMANCE**: dual-lane — fast actions (`dev/info`, `dev/settime`) bypass worker channel, slow actions tetap serial.
- **FService Proxy + Sync**: `ScanlogAllFull`, `ScanlogNew`, `SyncScanlog`, `SyncScanlogNew`, `SyncUsersFull`.
- **SDK Manager**: instance lifecycle — Start, Stop, Restart, Delete. Mutex-guarded. Kill chain: taskkill /F /T → terminateProcess.
- **Database Schema**: easylink.db v6, absen.db v2. Never DROP/recreate tables.
- **Event Logger**: ring buffer (500), SSE streaming. Close() broadcasts to all subscribers.
- **Device.ini Sync**: root Device.ini → bidirectional sync with DB. Symlink root → instances.

## 16. Protected Files (NEVER TOUCH WITHOUT EXPLICIT PLAN)

- `internal/services/syncer.go` — MODIFIED — PERFORMANCE (Phase 3: parallel sync + semaphore); Logging (Phase L: idle/success/warning/error logs)
- `internal/services/watchdog.go` — MODIFIED — PERFORMANCE (Phase 2: busy cooldown map)
- `internal/services/queue.go` — MODIFIED — PERFORMANCE (Phase 4: dual-lane fast/slow); Logging (Phase L: caller updates)
- `internal/services/sdk_manager.go` — PROVEN FINAL
- `internal/services/fservice.go` — MODIFIED — PERFORMANCE (Phase 1: connection pooling); Bug Fix (2026-06-25): SyncScanlog, SyncScanlogNew; Logging (Phase L): +logger param + per-page + sync start/done/error logs
- `internal/database/database.go` — NEVER CHANGE
- `internal/database/migrations.go` — MODIFIED — Phase I: v6 (user_sync_limit config)
- `internal/database/absen_migrations.go` — MODIFIED — Phase H: v2 (user_status column on device_info)
- `internal/config/config.go` — PROVEN FINAL
- `internal/services/process_windows.go` — NEVER CHANGE (Windows API wrappers)
- `internal/services/sync.go` — PROVEN FINAL
- `internal/services/logger.go` — PROVEN FINAL
- `internal/services/device_ini.go` — PROVEN FINAL
- `internal/services/setdef.go` — PROVEN FINAL
- `internal/services/sys_windows.go` — PROVEN FINAL
- `models/finger.go` — MODIFIED — Phase N: +NewPresensi field, +GetNewPresensi() method
- `core/` — READ-ONLY (Fingerspot master SDK)
- `template/` — DO NOT MANUALLY EDIT (embedded SDK copy)
- `gateway/main.go` — MODIFIED — PERFORMANCE (Phase 3: pass watchdog to NewSyncer)
- `internal/handlers/absen.go` — MODIFIED — Bug Fix (2026-06-25): HandleAbsenCompare, HandleAbsenScanlogSync; Phase H: HandleAbsenDeviceInfo +user_status, HandleAbsenSyncUsers +sdk_no path; Phase I: synced guard, skip_device param, config limit; Logging (Phase L): SyncUsersFull caller update
- `internal/handlers/scanlog.go` — MODIFIED — Bug Fix (2026-06-25): HandleScanlogAll
- `ui/js/app.js` — MODIFIED — Bug Fix Z (2026-06-25): anti-double-hit guards; Phase I: cache fields, polling skip_device, config save/load, synced guard; Phase N: device_scanlog in doSyncScanlog() body
- `templates/pages/users.html` — MODIFIED — Phase H: users page; Phase I: sync button synced guard + sync limit dropdown
- `templates/pages/scanlog.html` — MODIFIED — Phase I: sync button synced guard

## 17. Protected Functions (NEVER TOUCH WITHOUT EXPLICIT PLAN)

- `services.watchdog.Start()` — watchdog loop
- `services.watchdog.checkInstanceInfra()` — 3-stage validation
- `services.watchdog.checkDevicesForInstance()` — MODIFIED — PERFORMANCE: cooldown skip before POST
- `services.watchdog.checkDeviceHealth()` — MODIFIED — PERFORMANCE: MarkDeviceBusy on busy response
- `services.watchdog.IsDeviceInCooldown()` — NEW: exported cooldown check for syncer
- `services.watchdog.MarkDeviceBusy()` — NEW: exported busy marker for syncer
- `services.syncer.Start()` — syncer loop
- `services.syncer.tick()` — MODIFIED — PERFORMANCE: parallel goroutine + semaphore + WaitGroup
- `services.syncer.doDeviceSync()` — MODIFIED — PERFORMANCE: context parameter + cooldown check; Logging (Phase L): idle/success/warning/error logs
- `services.syncer.NewSyncer()` — MODIFIED — PERFORMANCE: added `*Watchdog` parameter
- `services.fservice.NewFServiceProxy()` — MODIFIED — PERFORMANCE: http.Transport pooling
- `services.queue.Enqueue()` — MODIFIED — PERFORMANCE: fast action bypass worker channel
- `services.queue.processJob()` — MODIFIED — PERFORMANCE: removed dev/info, dev/settime cases
- `services.fservice.SyncScanlog()` — MODIFIED — Bug Fix (2026-06-25): real COUNT(*), per-field dup check, event logger; Logging (Phase L): enhanced start/idle/done/error detail; Phase N: smart path (fast ScanlogNew → fallback ScanlogAllFull)
- `services.fservice.SyncScanlogNew()` — MODIFIED — Bug Fix (2026-06-25): per-field duplicate check; Logging (Phase L): +logger param + start/done/error logs
- `services.fservice.UserAllFull()` — MODIFIED — Phase I: default limit 100→30; Logging (Phase L): +logger param + per-page progress log
- `services.fservice.SyncUsersFull()` — MODIFIED — Phase H: user_status tracking; Phase I: default limit 100→30; Logging (Phase L): +logger param + start/done/error logs
- `services.fservice.ScanlogAllFull()` — MODIFIED — Logging (Phase L): +logger param + per-page progress log
- `services.sdk_manager.Start()` / `Stop()` / `Restart()` / `Delete()` — PROVEN FINAL
- `database.DB.AbsenMigrate()` — absen schema migration (PROVEN FINAL)
- `database.DB.Repair()` — idempotent data repair (PROVEN FINAL)
- `handlers.HandleAbsenScanlogSync()` — MODIFIED — Bug Fix (2026-06-25): pass logger to SyncScanlog; Phase I: synced guard (already_synced); Phase N: +device_scanlog bypass guard
- `handlers.HandleAbsenSyncUsers()` — MODIFIED — Phase H: sdk_no path; Phase I: config limit, synced guard
- `handlers.HandleAbsenCompare()` — MODIFIED — Bug Fix (2026-06-25): real COUNT(*); Phase I: skip_device param

## 18. Protected APIs (NEVER TOUCH WITHOUT EXPLICIT PLAN)

All API routes listed in Section 3 EXCEPT:
- `GET /api/devices/{sn}/absen/compare` — ALLOWED TO CHANGE (Phase C: sdk_no query param; Phase I: skip_device param)
- `GET /api/devices/{sn}/absen/info` — ALLOWED TO CHANGE (Phase H: +user_status field)
- `POST /api/devices/{sn}/users/sync` — ALLOWED TO CHANGE (Phase H: +sdk_no body; Phase I: +limit body, already_synced response)
- `POST /api/devices/{sn}/scan/sync` — ALLOWED TO CHANGE (Phase I: already_synced response; Phase N: +device_scanlog body)

## 19. UI Protection Registry

| Page | Route | Status |
|---|---|---|
| Dashboard | / | MIGRATED — Go template multi-page |
| Instances | /instances | MIGRATED — Go template multi-page |
| Devices | /devices | MIGRATED — Go template multi-page |
| Device Detail | /devices/{sn} | MIGRATED — Go template multi-page |
| Scanlog | /scanlog | ACTIVE (Phase C) |
| Users | /users | MIGRATED — Go template multi-page |
| Test | /test | MIGRATED — Go template multi-page |
| Jobs | /jobs | MIGRATED — Go template multi-page |
| Logs | /logs | MIGRATED — Go template multi-page |
| Settings | /settings | MIGRATED — Go template multi-page |

## 20. Deployment Protection Registry

- `build.ps1` — PROTECTED (build script)
- `gateway.exe` — PROTECTED (production binary)
- `instances/` — PROTECTED (runtime SDK instances)
- `logs/` — PROTECTED (runtime logs)

## 21. Database Protection Registry

- `easylink.db` — v6 schema, PROTECTED. Tables: schema_version, sdk_instances, devices, device_config, jobs, config.
- `absen.db` — v2 schema + Repair(), PROTECTED. Tables: schema_version, device_info (columns: sn, scanlog_count, user_count, scanlog_status, user_status, last_scan_sync, last_scan_check, last_user_sync, created_at, updated_at), scanlog, "user", template. Never DROP/recreate.

## 22. SDK Protection Registry

- FService.exe — binary from core/, never modify, never reverse-engineer
- template/ embedding — Go embed all:template, PROTECTED
- Device.ini format — [DEVICE{S/N}] sections, key=value, PROTECTED
- SetDef.fin format — port-only config, auto-generated, PROTECTED

## 23. Architecture Decisions (ALL FINAL)

All decisions in Section 8 remain FINAL.

### AD-027: User Sync Limit (FINAL)
- Default = 30, stored in `config` table as `user_sync_limit`
- Configurable via Users page dropdown (15/30/50/100)
- `HandleAbsenSyncUsers` reads config, fallback 30
- Scanlog limit tetap 100 (unchanged)

### AD-028: skip_device Query Param (FINAL)
- `GET /absen/compare?skip_device=1` returns `device: -1` tanpa hit mesin
- Digunakan frontend polling saat cache tersedia

### AD-029: Backend Synced Guard (FINAL)
- `POST /users/sync` + `POST /scan/sync`: return `{"status": "already_synced", ...}` jika local count == device count > 0
- Frontend disable Sync Now button saat synced

### AD-030: Sync Logging Standard (FINAL)
- Semua jalur sync wajib log start, progress (jika pagination), dan done/error dengan detail:
device count, local count, gap, inserted, total.
- Syncer otomatis: idle, success breakdown (+N new, before→after, device), warning fail, warning zero-inserted.
- Pagination (scanlog/users): per-page log (page=N got=M total=T, +done suffix on final page).
- Manual sync: start/idle/done/error dengan context device count, local count, inserted.
- Semua logger.Log() call nullable-safe (guard `if logger != nil`).

### AD-031: Smart Scanlog Sync (FINAL)
- Handler `POST /scan/sync` menerima optional `device_scanlog` untuk bypass stale guard
- `SyncScanlog` membaca `New Presensi` dari `dev/info` untuk memilih fast path (`ScanlogNew`) vs full path (`ScanlogAllFull`)
- Fast path: `newPresensi > 0` → `ScanlogNew` → jika gap tertutup → selesai → jika masih gap → `ScanlogAllFull`
- Full path: `newPresensi == 0` → langsung `ScanlogAllFull` (recovery saat buffer sudah dikonsumsi)
- Frontend otomatis kirim `device_scanlog` dari cache polling (field `_scanlogDeviceCount`)

