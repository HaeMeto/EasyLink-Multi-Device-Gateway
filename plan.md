# EasyLink Gateway — Implementation Plan

## Status: DONE (Phase D-N) ✅

---

## Summary

**Scope:** 5 bug fixes + 1 new feature (console log page)

| ID | Type | Title | Status |
|----|------|-------|--------|
| BUG-D1 | Bug | FService tidak clean-kill saat Stop/Restart | ✅ |
| BUG-D2 | Bug | Device.ini hanya terupdate di root, tidak sync instance | ✅ |
| BUG-D3 | Bug | Watchdog spam POST /dev/info dengan sn=0 | ✅ |
| BUG-D4 | Bug | Device sdk_no=0 tidak bisa digunakan | ✅ |
| FEAT-E1 | Feature | Console log page di UI + event logging backend | ✅ |

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

