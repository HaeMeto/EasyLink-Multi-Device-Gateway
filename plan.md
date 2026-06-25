# EasyLink Gateway — Implementation Plan

## Status: DONE (Phase D-W, X, R, S, C, F, G, H, Z, I) ✅

---

## Summary

**Scope:** 5 bug fixes + scanlog absen sync system + JSON fixes + UI overhaul + direct testing + sync comparison

| ID | Type | Title | Status |
|----|------|-------|--------|
| BUG-D1..D4 | Bug | FService clean-kill, Device.ini symlink, watchdog anti-spam, sdk_no=0 reroute | ✅ |
| FEAT-E1 | Feature | Console log page di UI + event logging backend | ✅ |
| FEAT-O | Feature | Scanlog absen sync system (absen.db + syncer + smart fetch) | ✅ |
| BUG-P1..P3 | Bug | JSON deserialization fixes (DEVINFO nested, scanlog/user case mismatch) | ✅ |
| FEAT-T | Feature | UI overhaul — Scanlog, Users, Settings pages | ✅ |
| FEAT-U | Feature | Bug fixes (handler resolution, template expand) + Testing page | ✅ |
| FEAT-V | Feature | Direct testing endpoint + sync comparison | ✅ |
| BUG-W | Bug | Compare undefined crash + template guards | ✅ |
| BUG-B1 | Bug | SyncScanlog skip sync when device_info.scanlog_count stale | ✅ |
| BUG-B2 | Bug | SyncScanlogNew no comprehensive duplicate check | ✅ |
| BUG-B3 | Bug | HandleAbsenCompare shows cached scanlog_count, not real COUNT | ✅ |
| BUG-B4 | Bug | HandleScanlogAll doesn't save to absen.db | ✅ |
| BUG-B5 | Bug | No success toast for Sync Now | ✅ |
| BUG-B6 | Bug | INSERT OR IGNORE only checks UNIQUE(sn,scan_date,pin), not all fields | ✅ |
| FEAT-H | Feature | Users page — model scanlog (instance dropdown, status badge, progress bar, compare, sdk_no, polling, toast) | ✅ |

---

## Phase 1: Project Bootstrap ✅
## Phase 2: Database Schema & Auto-Migration ✅
## Phase 3: Device.ini Parser & Generator + Sync ✅
## Phase 4: SDK Instance Manager ✅
## Phase 5: FService HTTP Proxy ✅
## Phase 6: Queue Manager ✅
## Phase 7: Watchdog & Auto Recovery ✅
## Phase 8: REST API Handlers ✅
## Phase 9: Web UI ✅
## Phase A: Critical Bug Fixes ✅
## Phase B: Instance Lifecycle & Sync Fixes ✅
## Phase C: UI Improvements + Quality Fixes ✅

---

## Phase D: Bug Fixes (5 items)

### D1: Clean Service Kill ✅
### D2: Symlink Device.ini Root → Instances ✅
### D3: Watchdog Anti-Spam ✅
### D4: Auto-Reroute sdk_no=0 ✅

## Phase E: Console Log Page (1 feature) ✅

### E1: Event Logger Backend ✅
### E2: Console Log UI Page ✅

**Files:** `ui/index.html`, `ui/js/app.js`

**Fix:**
- Nav item: Logs
- Log viewer: dark terminal-style, color-coded by log type
- Real-time SSE streaming from /api/logs/stream

---

## Execution Order

1. D1: Clean Service Kill
2. D2: Symlink Device.ini
3. D3: Watchdog Anti-Spam
4. D4: Auto-Reroute sdk_no=0
5. E1: Event Logger Backend
6. E2: Console Log UI Page
7. Build & Smoke Test

---

## Implementation Checklist

### BUG-D1
- [x] Stop(): `/F /T` tree kill + verify process dead + verify port released
- [x] Stop(): return error if kill/verification fails
- [x] Restart(): propagate error, pre-check port
- [x] recoverInstance(): same kill logic as Stop()

### BUG-D2
- [x] Create(): symlink root → instance Device.ini
- [x] SyncAfterDeviceChange(): remove per-instance sync
- [x] FullSync(): remove per-instance export loop
- [x] RestartAllRunning(): new method, called after device change
- [x] ExportToInstance, exportToInstanceLocked, syncAllInstanceInisLocked, syncAllInstanceInis: deleted
- [x] Handler device CRUD + SyncReload: call RestartAllRunning()

### BUG-D3
- [x] tickCount field + HTTP check only every 6 ticks

### BUG-D4
- [x] deviceLookup: fallback reroute sdk_no=0 to first RUNNING instance

### FEAT-E1
- [x] logger.go with ring buffer, SSE, broadcast
- [x] Routes /api/logs/stream and /api/logs
- [x] All logging points wired
- [x] UI Logs page functional with real-time updates

---

## Verified Behaviours

1. ✅ SetDef.fin auto-generated with correct port
2. ✅ All proxy endpoints work via queue manager
3. ✅ Queue serial per device, parallel across
4. ✅ Watchdog detects PID death via correct Windows API
5. ✅ Auto-recovery: kill → delete ldb → restart
6. ✅ Other instances unaffected during recovery
7. ✅ Web UI (SPA) accessible and functional
8. ✅ Alpine global store manages state and API calls
9. ✅ No changes to core/ or format of Device.ini/SetDef.fin
10. ✅ Backward compatible with existing EasyLink SDK
11. ✅ POST /api/sync/reload works
12. ✅ Status() never kills FService (OpenProcess API)
13. ✅ Database migration v2 is non-destructive
14. ✅ Auto-start on gateway restart
15. ✅ SQLite connection pool deadlock resolved

---

## Phase F: Audit Follow-Up ✅ (F-018, F-019, Q1-Q8)

### F-018 (FIXED): FService Root Endpoint Assumption
checkHTTP pakai HEAD `/` — tidak terverifikasi FService serve 200 di root.
**Fix**: Kembali ke POST `/dev/info` dengan SN valid (query device pertama via DB). Tambah 5s client timeout. Jika tidak ada device → skip HTTP check.

### F-019 (FIXED): memory.md + plan.md Tidak Terupdate
memory.md dan plan.md masih pre-implementasi.
**Fix**: Update mencerminkan Phase D, E, dan F. Semua bug fix, feature, struct changes, constructor signature changes, deleted functions terdokumentasi.

### Q1-Q8 (FIXED): Quality Gaps
- Q1: taskkill.Run() error checked di recoverInstance
- Q2: GetHealthReport tidak lagi panggil checkHTTP (PID+port only)
- Q3: RestartAllRunning async goroutine
- Q4: RestartAllRunning pakai EventLogger
- Q5: UI log type coloring (blue/yellow/green/red)
- Q6: Auto-scroll log container
- Q7: checkHTTP valid SN (bagian dari F-018)
- Q8: Import bytes cleanup

---

## Phase G: Delete & Kill Hardening ✅

### BUG-G1 (FIXED): Service Spawn Kembali (Watchdog Race)
Stop() update DB `status=STOPPED, pid=0` SEBELUM kill process. Jika kill gagal → rollback DB. Watchdog lihat status STOPPED → tidak trigger recovery.

### BUG-G2 (FIXED): FService.exe Zombie
Kill chain: `/F /T /PID` → verify (10x500ms) → `/F /IM FService.exe` → verify (5x500ms) → `TerminateProcess(pid)` via Windows API → error definitif jika semua gagal. New functions: `forceKill()`, `waitProcessDead()`, `terminateProcess()`.

### BUG-G3 (FIXED): Directory Tidak Terhapus
Delete() cek Stop() error + propagate. `os.RemoveAll` error dicek + retry 3x500ms. DB hanya dihapus jika RemoveAll sukses.

### BUG-G4 (FIXED): Device Disabled Saat Instance Dihapus
Hapus `enabled = 0` dari `UPDATE devices`. Device tetap enabled (sdk_no=0), bisa di-reroute via BUG-D4 logic.

### Files Modified
- `sdk_manager.go:190-245` → Stop() rewrite (DB-first + kill chain)
- `sdk_manager.go:258-275` → forceKill() + waitProcessDead() helpers
- `sdk_manager.go:117-160` → Delete() rewrite (error check + retry)
- `process_windows.go:11,16,38-51` → terminateProcess() + PROCESS_TERMINATE constant

---

## Phase H: Phase G Follow-Up ✅ (F-020, F-021, F-022, F-023)

### H1 (FIXED): Unify Kill Logic
forceKill() + waitProcessDead() dijadikan package-level function (bukan method *SdkManager). Watchdog.recoverInstance() memanggil forceKill() yang sama dengan Stop(). Kill chain 3-level sekarang dipakai di dua tempat — API stop dan watchdog recovery konsisten.

### H2 (FIXED): RestartAllRunning Race Window
Setelah Stop(), re-query DB status sebelum Start(). Jika status bukan STOPPED (user stop manual saat goroutine jalan), skip Start.

### H3 (FIXED): SysProcAttr on /IM Fallback
taskkill /IM FService.exe sekarang set SysProcAttr (console window hidden).

### H4 (FIXED): Documentation Update
memory.md dan plan.md diupdate mencerminkan Phase H.

### Files Modified
- `sdk_manager.go:248-281` → forceKill + waitProcessDead ke package-level
- `sdk_manager.go:229` → Stop() panggil forceKill() tanpa receiver
- `sdk_manager.go:350-367` → RestartAllRunning re-query DB sebelum Start
- `watchdog.go:199-211` → recoverInstance panggil forceKill() unified
- `watchdog.go:4-14` → hapus `os/exec`, `strconv` imports (unused)

---

## Phase I: Device Re-Enable ✅ (BUG-G5)

### BUG-G5 (FIXED): ImportFromRoot Tidak Re-enable Device yang Disabled
Old Delete code (pre-G4) set `enabled=0` pada device. `importFromRootLocked()` UPDATE tidak menyentuh kolom `enabled`. Device stuck invisible meskipun ada di root Device.ini.

**Fix:**
- `sync.go:125` — tambah `enabled=1` ke UPDATE statement importFromRoot
- `main.go:70` — one-time migration: `UPDATE devices SET enabled = 1 WHERE enabled = 0` setelah FullSync()

Idempotent, no-op jika semua device sudah enabled=1. Aman dijalankan setiap startup.

---

## Phase J: Graceful Shutdown ✅ (J1, J2, J3)

### J1 (IMPLEMENTED): EventLogger.Close()
**File:** `logger.go:86-93`
Tutup semua channel di subscribers map. SSE handler deteksi channel close → return → HTTP handler selesai → server bisa shutdown.

### J2 (IMPLEMENTED): Shutdown Sequence
**File:** `main.go:207-229`
Sequence: log shutdown → cancel watchdog → eventLogger.Close() → stop semua instance RUNNING (parallel goroutine + WaitGroup) → server.Shutdown(5s timeout).

### J3 (IMPLEMENTED): ListRunningSdkNos()
**File:** `sdk_manager.go:374-390`
Query `SELECT sdk_no FROM sdk_instances WHERE status = 'RUNNING'`. Return []int. Dipanggil dari main.go shutdown.

### Files Modified
- `logger.go:86-93` → Close() method
- `main.go:14-16` → import gosync + time
- `main.go:207-229` → shutdown: Close SSE → stop instances parallel → 5s server timeout
- `sdk_manager.go:374-390` → ListRunningSdkNos()

---

## Phase K: PID Auto-Detect & Recovery Refresh ✅ (K1, K2, K3)

**Problem:** FService.exe respawn dengan PID baru (FService internal watchdog / zombie child process). PID baru tidak tercatat di DB. Watchdog stuck pakai PID stale → `taskkill /F /T /PID <stale>` no-op → port masih occupied → loop recovery infinite.

### K1 (IMPLEMENTED): findPIDByPort — Get Real PID dari Port

**File:** `process_windows.go:60-88`

Function baru: `findPIDByPort(port int) (int, error)`

Jalankan `netstat -ano`, parse output, cari baris `LISTENING` pada port target, ekstrak PID dari kolom terakhir. Return PID, atau 0 jika tidak ditemukan.

### K2 (IMPLEMENTED): recoverInstance — Refresh Stale PID

**File:** `watchdog.go:197-210`

Sebelum panggil `forceKill()`, tambah logic:
1. Query PID dari DB
2. Jika pid > 0:
   - `isProcessAlive(pid)` → cek PID hidup
   - `checkPort(port)` → cek port open
   - Jika !alive && portOpen (PID stale):
     - `findPIDByPort(port)` → dapatkan PID real
     - `UPDATE sdk_instances SET pid = newPID` → refresh DB
     - Gunakan newPID untuk kill
3. `forceKill(pid, port)` — sekarang dengan PID benar

### K3 (IMPLEMENTED): forceKill — Final Safety Net via Port

**File:** `sdk_manager.go:275-288`

Setelah port check loop 10x gagal, tambah final fallback:
1. `taskkill /F /IM FService.exe` — kill semua FService
2. `time.Sleep(1s)`
3. Re-check port 5x @ 500ms
4. Jika masih open → return error definitif

### Files Modified
- `process_windows.go:3-12,60-88` → new imports + findPIDByPort()
- `watchdog.go:197-210` → PID refresh logic sebelum forceKill
- `sdk_manager.go:275-288` → /IM final fallback setelah port check gagal

### No Changes To
- Stop(), Restart(), Delete() — sudah benar
- Semua file lain

---

## Phase L: Smart Recovery & Targeted Restart ✅ (L1-L6)

### BUG-L1 (FIXED): Watchdog Kill Service Sehat
**Root cause:** checkInstance check PID dulu → PID stale → langsung trigger recovery → kill service yang baru start (port sebenarnya open).
**Fix:** Pindahkan PID refresh dari recoverInstance() ke checkInstance(). Jika PID stale tapi port open → findPIDByPort() + DB update → return nil (skip recovery).

### BUG-L2 (FIXED): RestartAllRunning Terlalu Broad
**Root cause:** Setiap device change trigger restart SEMUA instance. Device baru assign sdk-3 → sdk-1, sdk-2 juga restart tanpa alasan.
**Fix:** Hapus RestartAllRunning(). Ganti dengan targeted restart per-instance:
- HandleCreateDevice: restart req.SdkNo saja (>0)
- HandleUpdateDevice: restart oldSdkNo + newSdkNo (jika berbeda)
- HandleDeleteDevice: restart oldSdkNo (>0)
- HandleDeleteDeviceConfig: restart sdkNo terkait (>0)
- HandleUpdateDeviceConfig: restart sdkNo terkait (>0)
- HandleSyncReload: sequential restart semua RUNNING (single goroutine)

### BUG-L3 (FIXED): Watchdog vs Restart Race
**Root cause:** Setelah Start(), FService butuh ~2-3s bind port. Watchdog tick berikutnya bisa lihat port belum open → trigger recovery.
**Fix:** Tambah time.Sleep(3 * time.Second) setelah Start() di Restart().

### Files Modified
- `watchdog.go:117-145` → checkInstance: PID refresh + skip recovery
- `watchdog.go:102-114` → tick: enhanced unhealthy logging
- `watchdog.go:208-212` → recoverInstance: hapus PID refresh block
- `sdk_manager.go` → hapus RestartAllRunning() (38 lines deleted)
- `sdk_manager.go:339` → Restart: tambah 3s cooldown
- `handlers/device.go:89-92` → HandleCreateDevice: targeted Restart
- `handlers/device.go:118-163` → HandleUpdateDevice: query oldSdkNo + targeted restart
- `handlers/device.go:165-187` → HandleDeleteDevice: query oldSdkNo + targeted restart
- `handlers/device.go:218-232` → HandleUpdateDeviceConfig: query sdkNo + targeted restart
- `handlers/device.go:249-258` → HandleDeleteDeviceConfig: query sdkNo + targeted restart
- `handlers/handler.go:128-139` → HandleSyncReload: loop Restart via ListRunningSdkNos

### No Changes To
- process_windows.go, sync.go, queue.go, logger.go, main.go
- sdk_manager.go: Start/Stop/Delete/forceKill
- handlers/instance.go, handlers/handler.go lainnya

---

## Phase M: Multi-Stage Validation + Anti-Chain Recovery ✅ (M1-M5)

**Problem:** Watchdog model binary HEALTHY/UNHEALTHY. Tidak bisa bedakan service mati vs sibuk vs stale PID. Setelah recovery, status stuck ERROR — watchdog query `WHERE status = 'RUNNING'` tidak lihat instance.

### M1 (IMPLEMENTED): Set Status ERROR Sebelum Recovery
**File:** `watchdog.go:236`
`UPDATE SET status = 'ERROR' WHERE sdk_no = ?` dieksekusi sebelum Restart. Instance tidak muncul di tick berikutnya — maksimal 1 recovery per insiden.

### M2 (IMPLEMENTED): Gunakan Restart() Bukan forceKill + Start
**File:** `watchdog.go:228-256`
`recoverInstance()` sekarang panggil `sdkMgr.Restart(sdkNo)` — full flow dari SdkManager (Stop → port check → ldb remove → Start → 3s cooldown). Hapus forceKill, ldb remove, Start, sleep, checkHTTP manual.

### M3 (IMPLEMENTED): checkHTTP Parse Response, Kenali Busy Signal
**File:** `watchdog.go:166-213` + `errBusy` sentinel
Response body diparse JSON. Jika `{"Result":false,"message_code":3}` → return `errBusy`. Jika body valid → return nil. Timeout/non-200 → return error.

### M4 (IMPLEMENTED): checkInstance 3-Stage Validation
**File:** `watchdog.go:144-172`
Pipeline:
1. PID check → if fail + port open: refresh PID → skip recovery
2. Port check → if fail: trigger recovery
3. HTTP check (60s):
   - err == nil → set RUNNING → healthy
   - err == errBusy → log "busy" → healthy (don't recover)
   - err != nil → trigger recovery
4. LDB lock check

### M5 (VERIFIED): Status RUNNING Setelah Recovery
Restart() internally sets RUNNING via Start(). Tidak ada perubahan kode.

### Files Modified
- `watchdog.go:4-17` → add imports encoding/json, io + errBusy sentinel
- `watchdog.go:144-172` → checkInstance: 3-stage pipeline + errBusy handling
- `watchdog.go:166-213` → checkHTTP: JSON parse + errBusy return
- `watchdog.go:228-256` → recoverInstance: set ERROR + Restart() (hapus forceKill/Start/sleep/checkHTTP)

### No Changes To
- sdk_manager.go, device.go, handler.go, process_windows.go, sync.go, queue.go, logger.go, main.go
- Semua file lain

---

## Phase N: Instance Stability & Device Status ✅ (P1-P4)

### Phase 1: Remove Global Process Kill ✅
**Root cause:** `taskkill /F /IM FService.exe` di forceKill() membunuh semua instance.
**Fix:** Hapus kedua `/IM FService.exe` fallback. Ganti dengan `terminateProcess(pid)` retry 3x1s (kill phase) dan 5x1s (port phase). Kill hanya targeted per-PID.

### Phase 2: Watchdog Retry + Enhanced Logging ✅
**Changes:**
- `failCount map[int]int` di Watchdog struct — track consecutive failures per instance
- tick(): 5 consecutive failures baru trigger recovery. Busy (errBusy) tidak dihitung failure. Sukses reset counter.
- checkHTTP return (sn, err) — error message include SN dan URL lengkap
- Log format: `[fail X/5]` counter, `sn=` info

### Phase 3: Serialize Instance Operations ✅
**Changes:**
- `sync.Mutex` di SdkManager struct
- Start/Stop: public methods with Lock/Unlock → internal `startLocked()`/`stopLocked()`
- Restart(): Lock/Unlock + calls `stopLocked()`/`startLocked()` internally (no deadlock)

### Phase 4: Device Online/Offline Status ✅
**Changes:**
- Migration v3: `ALTER TABLE devices ADD COLUMN online INTEGER DEFAULT 1`
- Device model: field `Online int json:"online"`
- All device queries: SELECT/INSERT/UPDATE include online column
- Watchdog checkHTTP: query `AND online = 1`, skip if all devices offline
- UI: devices table shows Online/Offline badge

### Files Modified
- `sdk_manager.go:4-19` → import sync
- `sdk_manager.go:22-28` → mu sync.Mutex field
- `sdk_manager.go:158-169` → Start(): Lock + startLocked()
- `sdk_manager.go:211-222` → Stop(): Lock + stopLocked()
- `sdk_manager.go:249-299` → forceKill(): hapus /IM fallback, targeted terminateProcess retry
- `sdk_manager.go:330-370` → Restart(): Lock + stopLocked/startLocked
- `watchdog.go:4-20` → errBusy sentinel (no change from M)
- `watchdog.go:39-47` → failCount field
- `watchdog.go:55-62` → NewWatchdog init failCount
- `watchdog.go:106-123` → tick(): 5-strike retry + enhanced logging
- `watchdog.go:127-158` → checkInstance(): handle sn from checkHTTP
- `watchdog.go:177-215` → checkHTTP(): return (sn, err), online=1 filter
- `watchdog.go:228-253` → recoverInstance(): simplified signature, failCount reset
- `database/migrations.go:5` → currentVersion = 3
- `database/migrations.go:38-39` → case 3: migrateV3
- `database/migrations.go:144-147` → migrateV3(): ADD COLUMN online
- `models/device.go:14` → Online field
- `handlers/device.go:13` → Online in deviceCreateRequest
- `handlers/device.go:30,42,71-73,109-111,137-139` → all queries include online
- `ui/index.html:142-149` → Status column with Online/Offline badge

### No Changes To
- process_windows.go, sync.go, queue.go, fservice.go, logger.go, sys_windows.go, setdef.go, device_ini.go
- main.go, handler.go (except device handlers), instance.go, scanlog.go, user.go
- config.go, database.go
- ui/js/app.js, ui/css/app.css
- core/, template/

---

## Phase N (P5-P9): Device Health Separation & UI Toggle ✅

### Phase 5: Migration v4 — Device Health Columns ✅
**Changes:**
- `currentVersion = 4`
- `migrateV4()`: `ALTER TABLE devices ADD COLUMN fail_count INTEGER DEFAULT 0`, `ADD COLUMN last_offline TEXT`

### Phase 6: Device Model — New Fields ✅
**Changes:**
- `models/device.go`: `FailCount int json:"fail_count"`, `LastOffline string json:"last_offline"`

### Phase 7: Handler — Enable/Disable + Toggle Endpoint ✅
**Changes:**
- `deviceCreateRequest`: added `Enabled int json:"enabled"`
- `HandleListDevices`: SELECT/Scan add `fail_count`, `last_offline`
- `HandleCreateDevice`: INSERT adds `enabled` (default 1 if 0), `fail_count`, `last_offline`
- `HandleGetDevice`: SELECT/Scan add `fail_count`, `last_offline`
- `HandleUpdateDevice`: SET adds `enabled`, resets `fail_count=0, last_offline=''`
- `HandleToggleDevice` (NEW): `POST /api/devices/{id}/toggle`, flips enabled 0↔1, restarts instance if re-enabled
- `main.go`: registered toggle route

### Phase 8: Watchdog — Split Instance vs Device Health ✅
**Rewritten file:** `watchdog.go` (~120 lines changed)
- `failCount` → `instanceFailCount`
- `tick()` split: `queryRunningInstances()` → `checkInstanceInfra()` → `checkDevicesForInstance()`
- `checkInstanceInfra()`: PID → port → LDB lock, 5-strike before recovery, `[instance]` log prefix
- `checkDevicesForInstance()`: per-device HTTP check with `[device]` log prefix
  - Skips `enabled=0` devices
  - Skips offline devices with `last_offline` < 30min ago
  - Retry after 30min: `UPDATE online=1`
  - 5 consecutive HTTP failures → `online=0`, `last_offline=datetime('now')`
  - Busy signal (message_code=3) → not counted as failure
- `checkDeviceHealth()`: standalone function (port, sn) → POST /dev/info, returns errBusy for message_code=3
- Removed: `checkInstance`, `checkHTTP`, `checkPID`
- `recoverInstance`: `delete(w.instanceFailCount, sdkNo)` instead of `= 0`
- `GetHealthReport`: `HealthInstance` adds `DevicesOnline`, `DevicesTotal`. HTTPOk = `DevicesOnline > 0`. Pre-collects rows before nested queries to avoid SQLite deadlock.

### Phase 9: UI — Enable/Disable Toggle ✅
**Files:** `index.html`, `ui/js/app.js`
- Table: added "Enabled" checkbox column with `@change="toggleDeviceEnabled(d)"`
- Device form: added Enabled checkbox field
- `openDeviceForm`: includes `enabled`, `online` fields
- `doSaveDevice`: sends `enabled`, `online`
- `toggleDeviceEnabled(d)`: calls `POST /api/devices/{id}/toggle`

### Files Modified (Phases 5-9)
- `database/migrations.go`: v4 migration + currentVersion=4
- `models/device.go`: FailCount, LastOffline fields
- `handlers/device.go`: HandleToggleDevice + all query/scan updates
- `handlers/handler.go`: (read-only, no changes needed)
- `services/watchdog.go`: full refactor (instance/device health split)
- `main.go`: toggle route registration
- `ui/index.html`: Enabled column + form checkbox
- `ui/js/app.js`: toggleDeviceEnabled, form updates

### Bug Fix During Implementation
- **SQLite Deadlock in GetHealthReport**: `QueryRow` inside `rows.Next()` loop with `MaxOpenConns=1` caused deadlock. Fixed by pre-collecting rows into slice before closing rows and running nested queries.
- **SQLite Deadlock in checkDevicesForInstance**: Same pattern. Fixed by explicit `rows.Close()` before device loop instead of `defer`.

### Verified Behaviours
1. ✅ `fail_count`, `last_offline` columns exist in DB
2. ✅ API responses include new fields
3. ✅ Watchdog logs show `[instance]` and `[device]` prefixes
4. ✅ Device HTTP check runs every tick (no more 60s skip)
5. ✅ Instance health independent from device health
6. ✅ Toggle endpoint works (0↔1 flip)
7. ✅ SPA loads with `toggleDeviceEnabled` function
8. ✅ `go build` + `go vet` clean

---

## Phase O: Scanlog Absen Sync System ✅

### O1: Database Changes ✅
- **easylink.db migration v5**: `config` table (key, value, updated_at) with default `scanlog_sync_interval=60`
- **absen.db migration v1**: `device_info`, `scanlog`, `"user"`, `template` tables

### O2: Models (finger.go) ✅
Structs: `ScanlogEntry`, `ScanlogPagingResponse`, `DeviceInfoResponse`, `UserEntry`, `UserPagingResponse`, `TemplateEntry`, `AbsenDeviceInfo`, `ConfigEntry`

### O3: Config ✅
New field `AbsenDBPath` with env var `EASYLINK_ABSEN_DB_PATH`

### O4: FService Proxy ✅
- `ScanlogAllFull(port, sn, limit)` — session-based pagination loop
- `UserAllFull(port, sn, limit)` — session-based pagination loop
- `SyncScanlog(absenDB, port, sn)` — full sync (compare + fetch + insert)
- `SyncScanlogNew(absenDB, port, sn)` — incremental sync (dev/info + scanlog/new)
- `SyncUsersFull(absenDB, port, sn, limit)` — full user sync (DELETE + INSERT + templates)

### O5: Queue Actions ✅
- `scanlog/sync` — delegates to SyncScanlog
- `scanlog/sync-new` — delegates to SyncScanlogNew
- `user/sync-full` — delegates to SyncUsersFull
- Worker and QueueManager: added `absenDB *database.DB` field

### O6: Syncer (syncer.go) ✅
- Periodic goroutine (interval from config table, default 60s)
- Iterates enabled+online devices
- Compares dev/info count with absen.db device_info
- Fetches scanlog/all/paging (first sync) or scanlog/new (incremental)
- Inserts into absen.db scanlog table

### O7: Handlers ✅
- `absen.go`: HandleAbsenScanLogs, HandleAbsenDeviceInfo, HandleAbsenUsersList, HandleAbsenSyncUsers, HandleAbsenScanlogSync
- `config.go`: HandleGetConfig, HandlePutConfig
- `scanlog.go`: HandleScanlogNew (pure proxy, dipertahankan), HandleScanlogSmartFetch (smart fetch baru)

### O8: New Routes ✅
- `GET /api/devices/{sn}/scan/logs` — query absen.db scanlog with pagination
- `GET /api/devices/{sn}/scan/smart` — smart fetch (check stale → sync → return cache)
- `POST /api/devices/{sn}/scan/sync` — trigger manual scanlog sync
- `POST /api/devices/{sn}/users/sync` — trigger user sync
- `GET /api/devices/{sn}/absen/info` — device_info from absen.db
- `GET /api/devices/{sn}/absen/users` — user list from absen.db
- `GET /api/config` — list config entries
- `PUT /api/config` — update config entry

### O9: UI ✅
Dashboard: Sync Settings section with interval input + Save button

### Files Modified/Created
- `database/migrations.go` — v5 + currentVersion=5
- `database/absen_migrations.go` — NEW, absen migrations
- `models/finger.go` — NEW, scanlog/user/template/devinfo structs
- `config/config.go` — AbsenDBPath field + env var
- `services/fservice.go` — ScanlogAllFull, UserAllFull + sync methods
- `services/queue.go` — absenDB field, new sync actions
- `services/syncer.go` — NEW, periodic auto-sync
- `handlers/handler.go` — AbsenDB field
- `handlers/absen.go` — NEW, query + trigger sync
- `handlers/config.go` — NEW, GET/PUT config
- `handlers/scanlog.go` — smart fetch rewrite
- `main.go` — absen.db open/migrate, syncer start, new routes
- `ui/index.html` — sync settings section
- `ui/js/app.js` — syncInterval state, loadConfig/saveSyncInterval methods

### Verified Behaviours
1. ✅ easylink.db v5 migration creates config table with default interval
2. ✅ absen.db v1 migration creates 4 tables
3. ✅ GET /api/config returns config entries
4. ✅ PUT /api/config updates config
5. ✅ SPA loads with sync interval UI
6. ✅ `go build` + `go vet` clean
7. ✅ /health endpoint works
8. ✅ Gateway starts with both database migrations

---

## Phase P: JSON Deserialization & API Compatibility Fixes ✅

### P1 (FIXED): DeviceInfoResponse Nested DEVINFO Struct
**Root cause:** FService returns `{"Result":true,"DEVINFO":{"All Presensi":"2684","User":"759",...}}` — nested inside DEVINFO, values are string. Old struct parsed at root level with int type.
**Fix:** `DeviceInfoResponse` with nested `DEVINFO` struct (string fields) + `GetAllPresensi()` / `GetUser()` helper methods parsing string to int.

### P2 (FIXED): ScanlogEntry JSON Tags Uppercase
**Root cause:** FService returns uppercase keys (`"SN"`, `"ScanDate"`, `"PIN"`, etc.). Old Go struct used lowercase tags.
**Fix:** All `ScanlogEntry` JSON tags changed to uppercase: `SN`, `ScanDate`, `PIN`, `VerifyMode`, `IOMode`, `WorkCode`.

### P3 (FIXED): UserEntry/TemplateEntry JSON Tags Uppercase
**Root cause:** FService returns uppercase keys + `"Template"` (no 's'), `"idx"` for finger index. Old struct used lowercase with 's' and `"finger_idx"`.
**Fix:** All `UserEntry` tags uppercase (`PIN`, `Name`, `RFID`, `Password`, `Privilege`), `Templates` tag = `Template` (no 's'). `TemplateEntry` tags: `idx`, `pin`, `alg_ver`, `template`.

### P4 (FIXED): HandleScanlogNew Restored to Pure Proxy
**Root cause:** Phase O changed `HandleScanlogNew` from proxy to smart fetch, breaking API contract.
**Fix:** `HandleScanlogNew` restored to pure proxy → FService. New `HandleScanlogSmartFetch` handler registered at `GET /api/devices/{sn}/scan/smart` with proper error handling for sync failures.

### P5 (FIXED): Missing Route + Import Cleanup
- Registered `POST /api/devices/{sn}/scan/sync` → `HandleAbsenScanlogSync` (GAP-1)
- Removed duplicate `HandleAbsenSmartScanlogNew` from `absen.go`
- Removed unused imports (`encoding/json`, `time`) from `absen.go`
- `loadConfig()` now called in `init()` not just route `/`

### P6 (FIXED): Syncer Uses Correct Model Parsing
- `doDeviceSync` uses `models.DeviceInfoResponse` + `GetAllPresensi()` instead of anonymous struct
- Scanlog parsing uses `models.ScanlogPagingResponse` + `models.ScanlogEntry` with corrected tags

### P7 (FIXED): FService Sync Methods Use GetAllPresensi()
- `SyncScanlog` and `SyncScanlogNew` use `devInfo.GetAllPresensi()` instead of `devInfo.AllPresensi`

### Files Modified
- `models/finger.go` — all JSON tags uppercase, nested DEVINFO, helper methods, import strconv
- `services/fservice.go` — GetAllPresensi(), no other logic changes
- `services/syncer.go` — models.DeviceInfoResponse, GetAllPresensi()
- `handlers/scanlog.go` — restored HandleScanlogNew + new HandleScanlogSmartFetch
- `handlers/absen.go` — removed HandleAbsenSmartScanlogNew, removed unused imports
- `main.go` — registered GET .../scan/smart and POST .../scan/sync routes
- `ui/js/app.js` — loadConfig in init()

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ `/health` endpoint OK
3. ✅ `/api/config` GET/PUT OK
4. ✅ `GET /api/devices/{sn}/scan/smart` returns paginated data
5. ✅ `HandleScanlogNew` pure proxy preserved
6. ✅ All new routes registered

---

## Phase T: UI Overhaul — Scanlog, Users, Settings ✅

### T1: Settings Page (#/config) ✅
- Sync interval setting moved from Dashboard to dedicated Settings page
- Sidebar: "Settings" menu item (8th)
- Dashboard: removed sync settings widget

### T2: Scanlog Page (#/scanlog) ✅
- Device dropdown (from store.devices, sorted by SN)
- Sync status: total records, scanlog_status, last_scan_sync
- Sync Now button → POST /scan/sync → refresh table
- Data table: Scan Date, PIN, Verify, IO, Work Code
- Pagination: First/Prev/Page/Next/Last + page size selector
- Empty state: "No scanlog data. Click Sync Now to fetch."
- Syncing state: button disabled, "Syncing..." text

### T3: Users Page (#/users) ✅
- Device dropdown (reuse pattern from scanlog)
- User count + last_user_sync display
- Sync Now button → POST /users/sync (full replace)
- Data table: PIN, Name, RFID, Privilege, Tmpl (expand toggle)
- Pagination same as scanlog page

### T4: Device Detail Enhancement ✅
- "Scanlog All" button → set preselectedSN → navigate to #/scanlog
- "User All" button → set preselectedSN → navigate to #/users
- Other operations remain as raw JSON

### app.js Changes
- New state: `scanlogPage`, `usersPage`, `preselectedSN`
- New store methods: `fetchScanlogPage`, `fetchScanlogStatus`, `onScanlogDeviceChange`, `doSyncScanlog`
- New store methods: `fetchUsersPage`, `fetchUsersStatus`, `onUsersDeviceChange`, `doSyncUsers`
- `navigate()`: added #/scanlog, #/users, #/config cases
- #/scanlog and #/users: check preselectedSN → auto-populate dropdown

### Files Modified
- `ui/index.html` — 3 new sections, 3 sidebar items, removed dashboard sync settings, updated device detail buttons
- `ui/js/app.js` — scanlogPage, usersPage, preselectedSN state + all new methods

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ SPA serves with all 8 menu items
3. ✅ Scanlog page section present in HTML
4. ✅ Users page section present in HTML
5. ✅ Settings page section present in HTML
6. ✅ API endpoints work (/config, /scan/logs, /scan/sync, /absen/info, /absen/users, /users/sync)

---

## Phase U: Bug Fixes + Testing Page ✅

### U1 (FIXED): Event Handler Resolution
**Root cause:** Methods in `Alpine.reactive(store)` called without `store.` prefix from template. Alpine evaluates from `$data` root, so `store.` is required.
**Fix:** All `@change`, `@click` handlers in scanlog/users pages prefixed with `store.`. JS `navigate()` calls changed from `this.onScanlogDeviceChange()` to `this.store.onScanlogDeviceChange()`.

### U2 (IMPLEMENTED): Template Expand with Live Data
**Root cause:** Users page template expand showed hardcoded "No template data".
**Fix:**
- Backend: `HandleAbsenUserTemplates` handler → `GET /api/devices/{sn}/users/{pin}/templates` → queries template table by user_id
- Route registered in main.go
- Frontend: `store.fetchUserTemplates(sn, pin)` → fetch on Expand click → store result in `u._templates` → render finger_idx, alg_ver, template (truncated)

### U3 (IMPLEMENTED): Testing Page (#/test)
**Objective:** Quick API testing page with device selector and endpoint buttons.
- Sidebar: "Test" menu item (9th, between Users and Jobs)
- Device dropdown → auto-fills instance info (sdk_no, port, status)
- 8 API buttons: Device Info, Set Time, Init, Del Admin, Scanlog New, Scanlog All, User All, Log Del
- Response displayed in dark terminal-style textarea
- State: `testPage { sn, instanceInfo, result, loading }`

### U4 (FIXED): loadConfig double call + unnecessary fetchDevices in /config route
**Fix:** Removed `this.store.fetchDevices()` from `navigate('/config')` case.

### Files Modified
- `ui/index.html` — all store method calls prefixed, test page section, template expand dynamic fetch, sidebar +1
- `ui/js/app.js` — store. prefix in navigate, testPage state, onTestDeviceChange, runTest, fetchUserTemplates, removed fetchDevices from config route
- `handlers/absen.go` — HandleAbsenUserTemplates
- `main.go` — GET /api/devices/{sn}/users/{pin}/templates route

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ SPA serves with all 9 menu items (Dashboard + 8)
3. ✅ Scanlog handlers: `store.onScanlogDeviceChange`, `store.doSyncScanlog`, `store.fetchScanlogPage`
4. ✅ Users handlers: `store.onUsersDeviceChange`, `store.doSyncUsers`, `store.fetchUsersPage`
5. ✅ Template handler: `store.fetchUserTemplates` in SPA, template endpoint registered
6. ✅ Test page: `store.runTest`, `store.onTestDeviceChange`

---

## Phase V: Direct Testing + Sync Comparison ✅

### V1 (IMPLEMENTED): Direct Testing Endpoint
**Route:** `POST /api/test/device-info` — bypass queue, hit FService directly.
**Handler:** `HandleTestDeviceInfo` in `handlers/handler.go`.
**Logic:** Parse `{sn, sdk_no}` from body → query `sdk_instances` for port+status → validate RUNNING → call `Proxy.DeviceInfo(port, sn)` directly → return raw JSON.
**Handler struct:** added `Proxy *services.FServiceProxy` field.

### V2 (IMPLEMENTED): Sync Comparison Endpoint
**Route:** `GET /api/devices/{sn}/absen/compare`.
**Handler:** `HandleAbsenCompare` in `handlers/absen.go`.
**Logic:** Query `device_info` for local counts → queue `dev/info` for device counts → return `{scanlog: {local, device, synced}, users: {local, device, synced}, last_sync}`.

### V3 (IMPLEMENTED): Test Page Uses New Endpoint
`runDeviceInfo()` → POST `/api/test/device-info` with `{sn, sdk_no}` from dropdowns. Direct fetch, no queue.

### V4 (IMPLEMENTED): Scanlog/Users Pages Show Sync Comparison
- `fetchScanlogStatus()` and `fetchUsersStatus()` auto-call `/absen/compare` on device change
- Status bar shows: `Local: N | Device: M | Synced` (green) / `Mismatch (+X)` (yellow)
- Compare data stored in `scanlogPage.compare` and `usersPage.compare`

### Files Modified
- `handlers/handler.go` — Proxy field, testDeviceInfoRequest struct, HandleTestDeviceInfo
- `handlers/absen.go` — HandleAbsenCompare, added encoding/json import
- `main.go` — Handler Proxy field, 2 new routes
- `ui/js/app.js` — runDeviceInfo rewrite, compare in fetchScanlogStatus/fetchUsersStatus, compare state
- `ui/index.html` — scanlog/users status bar with compare display

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ `POST /api/test/device-info` route registered
3. ✅ `GET /api/devices/{sn}/absen/compare` route registered
4. ✅ JS: `runDeviceInfo` POSTs to new endpoint
5. ✅ JS: compare fetch auto-runs on device change
6. ✅ HTML: compare.scanlog/compare.users display in status bar

---

## Phase W: Compare Undefined Fixes ✅

### W1 (FIXED): Undefined Compare Crash
**Root cause:** `compare` in state init was `{}` — nested paths like `compare.scanlog.synced` crashed before async fetch populated data.
**Fix:** Init `compare` as `{scanlog: {}, users: {}}`. Wrap Synced/Mismatch spans with `<template x-if="compare.scanlog && compare.scanlog.device > 0">` so expressions not evaluated until data exists.

### W2 (FALSE ALARM): Double x-model @change
**Finding:** Alpine.js `x-model` on `<select>` does NOT fire DOM `@change` when value set programmatically. Explicit `onScanlogDeviceChange()` call in `navigate()` is necessary for preselectedSN flow. Reverted removal.

### W3 (VERIFIED): Data Display
Handler format `{total, page, size, data}` verified correct. JS `fetchScanlogPage` parses `d.data` / `d.total` correctly. Template renders `scan_date`, `pin`, `verify_mode`, `io_mode`, `work_code`. Data flow intact.

### Files Modified
- `ui/js/app.js:16-17` — compare init `{scanlog: {}, users: {}}`
- `ui/index.html:250-253,313-316` — x-if wrapper on Synced/Mismatch spans

---

## Phase X: Sync & Data Repair ✅

### Summary
**Problem:** Syncer auto-triggered full scanlog pagination every 60s for devices with `scanlog_count == 0`. Migration v2 (DROP/recreate scanlog table) rusak — ~20k record hilang, kolom `verify_mode`/`io_mode`/`work_code` selalu kosong.

**Fix:** (1) Syncer auto hanya `ScanlogNew` — full pagination manual via `POST /scan/sync`. (2) Hapus migration v2, rollback ke v1 + `Repair()` idempotent di startup. (3) `SyncScanlog` selalu `ScanlogAllFull` untuk disaster recovery.

### X1: Rollback Migration v2 → v1 + Repair ✅
- `absen_migrations.go:5` — `absenCurrentVersion = 1`
- `absen_migrations.go:36-37` — hapus `case 2: err = db.absenV2()`
- `absen_migrations.go:99-136` — hapus fungsi `absenV2()`
- `absen_migrations.go:97-109` — tambah `Repair()`: DELETE corrupt rows + UPDATE scanlog_count + SET scanlog_status='idle'
- `main.go:73-75` — panggil `absenDB.Repair()` setelah migrate

### X2: Fix Manual Sync → Always Full Pagination ✅
- `fservice.go:203-204` — `SyncScanlog` selalu `ScanlogAllFull(port, sn, 100)`, hapus branching `ScanlogNew`
- `fservice.go:210-217` — insert logic unified, selalu parse `ScanlogPagingResponse`

### X3: Data Fresh Start ✅
User action: hapus absen.db → restart gateway → Sync Now per device.

### Files Modified
- `database/absen_migrations.go` — rollback v2, tambah Repair()
- `main.go` — panggil Repair() startup
- `services/fservice.go` — SyncScanlog selalu full pagination

### Files NOT Touched (confirmed unchanged)
- `models/finger.go` — ✅ VerifyMode/IOMode/WorkCode already `int`
- `services/syncer.go` — ✅ auto syncer already uses `ScanlogNew` only
- `handlers/scanlog.go` — ✅ row struct already `int`
- `handlers/absen.go` — ✅ row struct already `int`
- `ui/index.html` — ✅ no changes needed
- `ui/js/app.js` — ✅ no changes needed
- `services/watchdog.go` — ✅ out of scope
- `services/queue.go` — ✅ out of scope
- `services/sdk_manager.go` — ✅ out of scope

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ `/health` endpoint OK (200)
3. ✅ Gateway starts with v1 migration + Repair() without error
4. ✅ All 5 instances RUNNING after startup
5. ✅ `GET /api/config` OK

---

## Phase R: Audit & Polish ✅

**Audit findings from Phase X implementation.** 3 minor items, none blocking.

### R1: Count Unification ✅
**Gap:** `SyncScanlog` dan `SyncScanlogNew` menulis `scanlog_count = allPresensi` (nilai device), sedangkan syncer menulis `COUNT(*)` (real rows). Jika UNIQUE constraint skip insert, count divergen.
**Fix:**
- `fservice.go:219-220` — `SyncScanlog`: query `SELECT COUNT(*) FROM scanlog WHERE sn = ?` → gunakan hasilnya, bukan `allPresensi`.
- `fservice.go:265-266` — `SyncScanlogNew`: perubahan yang sama.
- `fservice.go:227-234` — hapus fetch `DeviceInfo` di `SyncScanlogNew` (tidak dipakai lagi, `allPresensi` unused).

### R2: Error Feedback ✅
**Gap:** `doSyncScanlog()` catch block kosong. User tidak tahu jika sync gagal.
**Fix:**
- `app.js:189` — ganti `catch (e) {}` dengan toast error: `self.toast = { show: true, msg: 'Sync failed: ' + (e.message || e), type: 'error' }`.

### R3: Hapus Instance Selector ✅
**Gap:** Instance selector di scanlog page tidak difungsikan — `doSyncScanlog` tidak mengirim `sdkNo`. Membingungkan user.
**Fix:**
- `index.html:245-253` — hapus `<div>` instance selector.
- `app.js:16` — hapus field `sdkNo` dari `scanlogPage` state.

### Files Modified
- `services/fservice.go` — COUNT(*) unification + hapus dead DeviceInfo di SyncScanlogNew
- `ui/js/app.js` — error toast di doSyncScanlog + hapus sdkNo state
- `ui/index.html` — hapus instance selector scanlog page

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ `/health` endpoint OK (200)
3. ✅ Toast component present for error feedback

---

## Phase S: Instance Selector for Scanlog Sync ✅

**Objective:** Restore instance selector di scanlog page dengan wiring ke direct sync (bypass queue). User dapat memilih instance spesifik untuk manual sync.

### S1: Restore Instance Selector UI ✅
- `index.html:245-253` — restore instance selector div (dropdown + Auto option) di scanlog page flex row, posisi setelah device selector
- `app.js:16` — restore field `sdkNo: ''` ke `scanlogPage` state

### S2: Wire SDK Number to Request ✅
- `app.js:188` — `doSyncScanlog()` kirim JSON body `{ sdk_no: parseInt(self.scanlogPage.sdkNo) \|\| 0 }`

### S3: Handle Instance Selection di Backend ✅
- `absen.go:192-241` — `HandleAbsenScanlogSync` parse `sdk_no` dari body:
  - `sdk_no > 0`: query `sdk_instances` → validasi RUNNING + port > 0 → `h.Proxy.SyncScanlog(h.AbsenDB, port, sn)` langsung (bypass queue)
  - `sdk_no == 0` / no body: path existing → `h.Queue.Enqueue(sn, "scanlog/sync", nil)` (backward compatible)
  - Error handling: instance not found (404), not running (503), invalid port (503)

### Files Modified
- `ui/index.html` — restore instance selector
- `ui/js/app.js` — restore sdkNo state + wiring sdk_no to POST body
- `handlers/absen.go` — parse sdk_no + branch direct/queue

### Files NOT Touched
- `fservice.go`, `syncer.go`, `queue.go`, `handler.go`, `watchdog.go` — no changes

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ `/health` endpoint OK (200)
3. ✅ Instance selector appears in scanlog page SPA (device + instance dropdowns)
4. ✅ `POST /scan/sync` with `{"sdk_no":99}` → "instance not found" error
5. ✅ `POST /scan/sync` with `{"sdk_no":0}` → queue path (backward compatible)
6. ✅ `POST /scan/sync` (no body) → queue path (backward compatible)

---

## Phase C: Compare Button + Instance-Aware Compare Endpoint ✅

### Summary
Add explicit Compare button to scanlog page that respects instance selector. Separate auto-compare from user-triggered compare. Backend `HandleAbsenCompare` accepts `sdk_no` query param.

### Phase 0: Memory Hardening ✅
- `memory.md` Sections 15-23 added: Stable Areas, Protected Files, Protected Functions, Protected APIs, UI Protection Registry, Deployment Protection Registry, Database Protection Registry, SDK Protection Registry, Architecture Decisions (ALL FINAL).

### C1: Backend — HandleAbsenCompare Accepts sdk_no ✅
- `absen.go:291-352` — `HandleAbsenCompare` reads `sdk_no` from query string:
  - `sdk_no > 0`: direct `h.Proxy.DeviceInfo(port, sn)` bypass queue (pattern from HandleTestDeviceInfo). Validates RUNNING + port > 0.
  - `sdk_no == 0` / absent: existing queue path `h.Queue.Enqueue(sn, "dev/info", nil)`.
  - Response parsing unchanged (DEVINFO.AllPresensi, DEVINFO.User).

### C2: UI — Add Compare Button ✅
- `index.html:254-259` — Compare button (green bg-green-600) in scanlog page flex row, between instance selector and flex row close. Disabled when no device or comparing.

### C3: JS — State + Split fetchScanlogStatus + doCompare ✅
- `app.js:16` — added `comparing: false` to scanlogPage state
- `app.js:162-173` — `fetchScanlogStatus(sdkNo)`: accepts optional param. sdkNo undefined → skip compare (info-only). sdkNo provided → `/absen/compare?sdk_no=${sdkNo}`
- `app.js:174-180` — `onScanlogDeviceChange()`: no auto-compare (calls fetchScanlogStatus without sdkNo)
- `app.js:210-220` — `doCompare()`: new method, sets comparing, calls fetchScanlogStatus with sdkNo, error toast
- `app.js:222-226` — `startProgressPoll()`: passes sdkNo to fetchScanlogStatus during sync
- `app.js:195-196` — `doSyncScanlog()`: passes sdkNo to fetchScanlogStatus after sync

### Files Modified
- `memory.md` — Sections 15-23 (protection registries)
- `handlers/absen.go` — HandleAbsenCompare: sdk_no branching
- `ui/index.html` — Compare button in scanlog flex row
- `ui/js/app.js` — comparing state, fetchScanlogStatus(sdkNo), doCompare, poll update

### Files Protected (NOT touched)
- syncer.go, watchdog.go, queue.go, sdk_manager.go, fservice.go, database.go, migrations.go, absen_migrations.go, config.go, handler.go, main.go, scanlog.go, user.go, device.go, instance.go, all other handlers

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ `/health` endpoint OK (200)
3. ✅ Compare button in SPA (green, disabled when no device)
4. ✅ `GET /absen/compare` (no param) → queue path (backward compatible)
5. ✅ `GET /absen/compare?sdk_no=0` → queue path
6. ✅ `GET /absen/compare?sdk_no=99` → "instance not found"
7. ✅ All 9 menu items present in SPA

---

## Phase F: Scanlog Page Missing Instance List ✅

**Problem:** Instance dropdown di scanlog page hanya menampilkan "Auto" — `fetchInstances()` tidak dipanggil saat navigasi ke `#/scanlog`.

**Fix:** Satu baris di `navigate()` function.

### F1: Add fetchInstances() to Scanlog Navigation ✅
- `app.js:336` — tambah `this.store.fetchInstances();` setelah `this.store.fetchDevices();` dalam case `#/scanlog`

### Files Modified
- `ui/js/app.js` — line 336

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ `/health` endpoint OK (200)
3. ✅ `navigate()` case scanlog loads both `fetchDevices()` + `fetchInstances()`
4. ✅ Instance selector template unchanged in index.html

---

## Phase G: Scanlog Sync Bug Fixes (6 items) ✅

**Date:** 2026-06-25

### BUG-B1: SyncScanlog skip sync when device_info.scanlog_count stale ✅
**File:** `internal/services/fservice.go:175-245`
**Fix:** Replace `info.ScanlogCount == allPresensi` comparison with real `SELECT COUNT(*) FROM scanlog WHERE sn=?`. Added `&& allPresensi > 0` guard.

### BUG-B2 & BUG-B6: SyncScanlogNew per-field duplicate check ✅
**File:** `internal/services/fservice.go:247-289`
**Fix:** Replaced `INSERT OR IGNORE` with explicit `SELECT COUNT(*) WHERE sn=? AND scan_date=? AND pin=? AND verify_mode=? AND io_mode=? AND work_code=?`. Only INSERT if count=0.

### BUG-B1 (duplicate check): SyncScanlog per-field duplicate check ✅
**File:** `internal/services/fservice.go:220-232`
**Fix:** Same per-field duplicate check as SyncScanlogNew. Response includes `"inserted"` field.

### BUG-B3: HandleAbsenCompare real COUNT(*) ✅
**File:** `internal/handlers/absen.go:302-312`
**Fix:** Changed `SELECT scanlog_count, user_count, ... FROM device_info` to `SELECT COUNT(*) FROM scanlog WHERE sn=?` for localScanlog. user_count still read from device_info.

### BUG-B4: HandleScanlogAll save to absen.db ✅
**File:** `internal/handlers/scanlog.go:109-158`
**Fix:** After queue proxy response, parse JSON and insert each entry to absen.db scanlog table with per-field duplicate check. Added `"fmt"` import.

### BUG-B5: doSyncScanlog success toast ✅
**File:** `gateway/ui/js/app.js:182-201`
**Fix:** Parse fetch response JSON, show success toast with count/inserted info. Keep error toast in catch block.

### Logger parameter addition
- `SyncScanlog` signature changed: added `logger *EventLogger` parameter
- `absen.go:226`: passes `h.Logger` to SyncScanlog
- `queue.go:264`: passes `w.logger` to SyncScanlog
- Event logs: sync start (device=%d local=%d), sync error (%v), sync done (%d inserted, total=%d)
- `HandleScanlogAll`: logs `scanlog/all saved: %d records`

### Files Modified
- `gateway/internal/services/fservice.go` — SyncScanlog (signature + body), SyncScanlogNew (body)
- `gateway/internal/handlers/absen.go` — HandleAbsenCompare (count query), HandleAbsenScanlogSync (pass logger)
- `gateway/internal/handlers/scanlog.go` — HandleScanlogAll (insert to absen.db), added `fmt` import
- `gateway/internal/services/queue.go` — line 264 (pass logger to SyncScanlog)
- `gateway/ui/js/app.js` — doSyncScanlog (parse response + success toast)

### memory.md Updated
- Section 16: Added 5 files as MODIFIED — Bug Fix
- Section 17: Changed 3 functions from PROVEN FINAL to MODIFIED

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ SyncScanlog uses real `COUNT(*)` from scanlog table
3. ✅ SyncScanlog & SyncScanlogNew do per-field (6 columns) duplicate check
4. ✅ HandleAbsenCompare reads `COUNT(*)` for local scanlog
5. ✅ HandleScanlogAll saves data to absen.db as side effect
6. ✅ doSyncScanlog shows success toast with record count
7. ✅ Event logger called for sync start/error/done
8. ✅ All callers updated for new SyncScanlog signature

---

## Phase G-1: Fix HandleScanlogAll device_info initialization ✅

**Date:** 2026-06-25  
**Bug:** B-A1 — `HandleScanlogAll` UPDATEs `device_info.scanlog_count` without ensuring the row exists first. If user presses "Scan Log All" before "Sync Now", the UPDATE is a silent no-op. `SyncScanlog` already does `INSERT OR IGNORE` before UPDATE; `HandleScanlogAll` now does the same.

### Fix
- `scanlog.go:157` — added `INSERT OR IGNORE INTO device_info (sn, scanlog_count, user_count) VALUES (?, 0, 0)` before the UPDATE, matching the pattern in `SyncScanlog` (fservice.go:185-188)

### Files Modified
- `gateway/internal/handlers/scanlog.go` — 1 line added

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ First-time "Scan Log All" before "Sync Now" → device_info row created
3. ✅ Idempotent: no error if row already exists (INSERT OR IGNORE)

---

## Phase H: Users Page — Model Scanlog ✅

**Objective:** Mengadopsi model halaman scanlog ke halaman users — instance dropdown, status badge, progress bar, compare button, polling saat sync, toast messages, dan sdk_no support di backend.

### H1: Database — Migration v2 ✅
- `absen_migrations.go`: `absenCurrentVersion=2`, case 2 → `absenV2()`
- `absenV2()`: `ALTER TABLE device_info ADD COLUMN user_status TEXT DEFAULT 'idle'`

### H2: Backend — HandleAbsenDeviceInfo ✅
- Struct info: +`UserStatus string json:"user_status"`
- SELECT: +`user_status`, Scan: +`&info.UserStatus`
- Fallback: `"user_status": "idle"`

### H3: Backend — HandleAbsenSyncUsers (sdk_no) ✅
- Parse JSON body `{sdk_no: int}`
- sdk_no > 0: query sdk_instances → validate → direct `h.Proxy.SyncUsersFull(port, sn, 100)`
- sdk_no == 0 / no body: existing queue path

### H4: Backend — SyncUsersFull (user_status tracking) ✅
- Start: `INSERT OR IGNORE` + `UPDATE user_status='syncing'`
- End: `user_status='idle'` in both INSERT/UPDATE paths

### H5: Frontend — users.html ✅
- Instance dropdown (`x-model="usersPage.sdkNo"`)
- Status badge (`user_status` color-coded: syncing=blue, stale=red, idle=green)
- Progress bar (`(local/device)*100%`)
- Compare button + Sync Now button
- Last sync timestamp

### H6: Frontend — app.js ✅
- State: `+sdkNo`, `+comparing`, `+_pollTimer`
- `doSyncUsers()`: sdk_no body, toast success/error, polling
- `fetchUsersStatus(sdkNo)`: optional sdkNo → query param to compare
- `doCompareUsers()`: new, trigger compare
- `startUsersProgressPoll()` / `stopUsersProgressPoll()`: new, 2s interval
- `onUsersDeviceChange()`: +stop poll, reset syncing
- init: `+store.fetchInstances()` for /users

### Files Modified
- `gateway/internal/database/absen_migrations.go`
- `gateway/internal/handlers/absen.go`
- `gateway/internal/services/fservice.go`
- `gateway/templates/pages/users.html`
- `gateway/ui/js/app.js`

### APIs Modified
- `GET /api/devices/{sn}/absen/info` — +`user_status` field
- `POST /api/devices/{sn}/users/sync` — +optional `sdk_no` body

### Verified Behaviours
1. ✅ `go build` + `go vet` clean
2. ✅ `build.ps1` production build success
3. ✅ Binary boots, migration v2 applied, "absen database migrated ok"
4. ✅ Backward compatible — no body / sdk_no=0 uses queue path
5. ✅ Protected areas untouched: syncer, watchdog, queue, sdk_manager, scanlog.html


---

## Phase Z: Anti-Double-Hit Guards — DONE ✅

### Z1: Guard `init()` — cegah eksekusi ganda ✅
- `init()` line 24-25: `_initialized` guard prevents re-execution

### Z2: Deduplikasi concurrent API request di `api()` ✅
- Store field `_pending: {}` + dedupe logic in `api()` (lines 33-57)
- Key = `method + ':' + path`, promise reused if pending

### Z3: Hapus `fetchHealth()` ganda di branch `/devices` ✅
- Removed duplicate `store.fetchHealth()` from `/devices` branch (was line 358)

### Z4: Guard `startLogStream()` — cegah double EventSource ✅
- `startLogStream()` line 70: closes existing `_es` before creating new EventSource

### Files Modified
- `gateway/ui/js/app.js`

### Functions Modified
- `init()` — +2 lines guard
- `api()` — dedupe wrapper (+~12 lines)
- `startLogStream()` — +1 line guard
- `init()` route `/devices` — -1 line

### Build Result
- `go build`: clean
- `go vet ./...`: clean

### Validation
- All 4 guards in place
- No function signatures changed
- No template/HTML changes
- No backend changes
- Backward compatible

---

## Phase I: User Sync Optimisasi + UI Guards — DONE ✅

### Overview
Optimisasi user sync: turunkan limit paging dari 100 ke 30 (configurable via `config` table + UI dropdown), tambah `skip_device` param di compare endpoint, guard backend sync endpoint tolak sync jika sudah synced, UI disable tombol Sync Now jika sudah synced, cache device count di Alpine store.

### Phase 1 — Database Migration + Backend Limit Config ✅
- Migration v6: `INSERT OR IGNORE INTO config (key, value) VALUES ('user_sync_limit', '30')`
- `UserAllFull` + `SyncUsersFull`: default limit 100→30
- `HandleAbsenSyncUsers`: baca `user_sync_limit` config, fallback 30, oper ke `SyncUsersFull` + `Queue.Enqueue`

### Phase 2 — Backend skip_device + Synced Guard ✅
- `HandleAbsenCompare`: support `skip_device=1` query param → return `device: -1` tanpa hit mesin
- `HandleAbsenSyncUsers`: guard — tolak jika `user_count == COUNT("user") > 0`, return `already_synced`
- `HandleAbsenScanlogSync`: guard — tolak jika `scanlog_count == COUNT(scanlog) > 0`, return `already_synced`

### Phase 3 — Frontend Cache + Polling Optimisasi ✅
- Alpine store: tambah `_scanlogDeviceCount` + `_usersDeviceCount`
- `fetchScanlogStatus` / `fetchUsersStatus`: support `skip_device` + cache
- `startProgressPoll` / `startUsersProgressPoll`: otomatis gunakan skip_device via cache
- `doSyncScanlog` / `doSyncUsers`: reset cache + full compare setelah sync
- `doCompare` / `doCompareUsers`: force full hit + cache
- `onScanlogDeviceChange` / `onUsersDeviceChange`: reset cache + load config

### Phase 4 — UI Changes ✅
- `users.html`: Sync Now `:disabled` + `synced`, dropdown limit 15/30/50/100
- `scanlog.html`: Sync Now `:disabled` + `synced`
- `doSyncUsers`: kirim `limit` dalam body request
- `saveUserSyncLimit()` + `loadUserSyncLimit()`: auto-save ke `/api/config`

### Files Modified
- `gateway/internal/database/migrations.go`
- `gateway/internal/handlers/absen.go`
- `gateway/internal/services/fservice.go`
- `gateway/ui/js/app.js`
- `gateway/templates/pages/users.html`
- `gateway/templates/pages/scanlog.html`

### APIs Modified
- `GET /api/devices/{sn}/absen/compare` — +`skip_device` query param
- `POST /api/devices/{sn}/users/sync` — +`limit` body field, response boleh `already_synced`
- `POST /api/devices/{sn}/scan/sync` — response boleh `already_synced`

### Architecture Decisions
- **AD-027**: User sync limit default = 30, stored in `config` table as `user_sync_limit`
- **AD-028**: Compare endpoint support `skip_device` query param
- **AD-029**: Backend guard: tolak sync jika data sudah match

### Build Result
- `go build ./...`: clean ✅
- `go vet ./...`: clean ✅
- `build.ps1`: success ✅

### Validation Checklist
- [x] Migration v6: `user_sync_limit = '30'` di config table
- [x] `SyncUsersFull` + `UserAllFull` default limit 30
- [x] `HandleAbsenSyncUsers` baca config + guard already_synced
- [x] `HandleAbsenScanlogSync` guard already_synced
- [x] `HandleAbsenCompare` skip_device=1 → device=-1
- [x] Cache device count di Alpine store
- [x] Polling gunakan skip_device saat cache tersedia
- [x] Reset cache setelah sync + device change
- [x] Disable Sync Now saat synced (both pages)
- [x] Sync limit dropdown users page
- [x] Semua Protected Files untouched ✅
- [x] Backward compatible ✅

---

## Phase L: Sync Logging Detail — DONE ✅

### Status: APPROVED (Audited 2026-06-25)

### Summary
Tambahkan logging detail di semua jalur sync: syncer otomatis, sync pagination scanlog, sync pagination user.

### Files Modified
- `gateway/internal/services/syncer.go` — idle/success/warning/error logs in `doDeviceSync()`
- `gateway/internal/services/fservice.go` — +logger param + per-page + sync start/done/error logs in 5 functions
- `gateway/internal/services/queue.go` — caller updates for SyncScanlogNew/SyncUsersFull
- `gateway/internal/handlers/absen.go` — caller update for SyncUsersFull

### Functions Modified
- `ScanlogAllFull()` — +logger param + per-page progress log
- `UserAllFull()` — +logger param + per-page progress log
- `SyncScanlog()` — enhanced start/idle/done/error detail
- `SyncScanlogNew()` — +logger param + start/done/error logs
- `SyncUsersFull()` — +logger param + start/done/error logs
- `doDeviceSync()` — idle/success/warning/error logs

### APIs Modified
None.

### Architecture Decisions
- **AD-030**: Sync Logging Standard (FINAL) — Semua jalur sync wajib log start/progress/done/error.

### Build Result
- `go build ./...`: clean ✅
- `go vet ./...`: clean ✅
- `build.ps1`: success ✅ (copy blocked by running binary)

### Audit Result
- Approved (zero code change required)
- 4/4 files match plan
- 6/6 functions match plan
- 0 scope violations
- 0 protected area violations

### Validation Checklist
- [x] Syncer idle log muncul saat tidak ada data baru
- [x] Syncer success log muncul dengan breakdown (+N new, before→after, device)
- [x] Syncer warning log muncul saat device has gap but 0 inserted
- [x] Syncer error log enhanced dengan device/db context
- [x] SyncScanlog start/idle/done/error log detail enhanced (+gap, +device)
- [x] Scanlog paging per-page log (page=N got=M total=T)
- [x] Users paging per-page log (page=N got=M total=T)
- [x] SyncScanlogNew start/done/error logs
- [x] SyncUsersFull start/done/error logs
- [x] Semua logger.Log() call nullable-safe (guard `if logger != nil`)
- [x] Zero behavior change di semua jalur sync
- [x] Zero API change
- [x] Zero UI change
- [x] All Protected Files untouched
- [x] Backward compatible ✅

---

## Phase N: Smart Scanlog Sync — Anti-False-Positive Guard + Fast Path — DONE ✅

### Overview

Bug: `HandleAbsenScanlogSync` guard (`already_synced`) menghasilkan false positive saat `scanlog_count` (cached syncer) sudah equal dengan `COUNT(*)` tapi device masih punya data lebih banyak. Entry yang hilang tersedia di `scanlog/all` tapi `New Presensi = 0` (buffer sudah dikonsumsi oleh `scanlog/new` sebelumnya).

Solusi: 3 perubahan backend + 1 frontend.

### Phase 1 — Models: Parse `New Presensi` — DONE ✅
- `finger.go`: `NewPresensi string json:"New Presensi"` field + `GetNewPresensi() int` method

### Phase 2 — Handler: `device_scanlog` guard bypass — DONE ✅
- `absen.go`: `DeviceScanlog int json:"device_scanlog"` field + bypass when `device_scanlog > localCount`

### Phase 3 — SyncScanlog: Smart fast → full fallback — DONE ✅
- `fservice.go`: check `newPresensi`, fast path via `ScanlogNew` if >0, full fallback via `ScanlogAllFull`

### Phase 4 — Frontend: Auto-send `device_scanlog` — DONE ✅
- `app.js`: include `device_scanlog` from `_scanlogDeviceCount` cache in `doSyncScanlog()` body

### Phase 5 — Reference: JSON samples in plan.md — DONE ✅
- Appendix A added

### Files Modified
- `gateway/internal/models/finger.go`
- `gateway/internal/handlers/absen.go`
- `gateway/internal/services/fservice.go`
- `gateway/ui/js/app.js`
- `plan.md`

### APIs Modified
- `POST /api/devices/{sn}/scan/sync` — +optional `device_scanlog` int body field

### Protected Areas Verified
- syncer.go — untouched
- watchdog.go — untouched
- sdk_manager.go — untouched
- queue.go — untouched
- database/ — untouched
- config/ — untouched
- templates/ — untouched
- main.go — untouched

---

## Appendix A: FService JSON Response Formats

### dev/info

```json
{
 "Result": true,
 "DEVINFO": {
 "All Presensi": "2675",
 "New Presensi": "10",
 "User": "150"
 }
}
```

Key fields:
- `All Presensi`: total scanlog entries on device
- `New Presensi`: entries in new-entry buffer (reset after `scanlog/new` retrieval)
- `User`: total user entries on device

### scanlog/new

```json
{
 "Result": true,
 "IsSession": false,
 "Data": [
 {
 "SN": "DEV123",
 "ScanDate": "2025-06-25 10:30:00",
 "PIN": "001",
 "VerifyMode": 15,
 "IOMode": 1,
 "WorkCode": 0
 }
 ]
}
```

Key notes:
- Buffer entries; need `scanlog/del` to flush after retrieval
- May be empty even when `New Presensi > 0` if buffer already consumed

### scanlog/all/paging

```json
{
 "Result": true,
 "IsSession": true,
 "Data": [
 {
 "SN": "DEV123",
 "ScanDate": "2025-06-25 09:15:00",
 "PIN": "002",
 "VerifyMode": 15,
 "IOMode": 1,
 "WorkCode": 0
 }
 ]
}
```

Key notes:
- `IsSession: true` means there are more pages; paginate until `IsSession: false`
- Uses `scanlog/all/paging` endpoint with `limit` and page-based pagination

### Audit Resolution (2026-06-25)

| Finding | Fix | Status |
|---------|-----|--------|
| B1: `inserted` undercount in fallback path log + JSON response | Use `newCount - localCount` in both `SyncScanlog` log and response | FIXED |
| B2: `SyncScanlogNew` had `newCount - localCount` (pre-existing, `localCount` undefined) | Replaced with `inserted` | FIXED |
| Memory duplicate: `fservice.go` at line 537 | Removed | FIXED |
| Memory duplicate: `queue.go` at line 540 | Removed | FIXED |
| Phase N header `IN PROGRESS` | Changed to `DONE ✅` | FIXED |

- `go build ./...` clean
- `go vet ./...` clean
- `build.ps1` success
