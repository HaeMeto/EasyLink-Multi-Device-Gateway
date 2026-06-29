# EasyLink Gateway

**Multi-Device Gateway untuk Fingerspot EasyLink SDK**

EasyLink Gateway adalah wrapper untuk EasyLink SDK Fingerspot yang mengatasi masalah SDK bawaan saat mengakses banyak device secara bersamaan — sering mengalami **hang**, **error**, atau **busy**. Gateway ini menyediakan antrian serial per-device, health monitoring dengan auto-restart, serta web dashboard dan REST API untuk manajemen terpusat device absensi sidik jari, sinkronisasi data absensi, dan pemantauan real-time.

> **Platform:** Windows only · **Bahasa:** Go 1.25 · **Database:** SQLite

---

## Status Pengembangan

Fokus pengembangan saat ini adalah:

- **Manajemen SDK/instance & devices** — tambah, start, stop, restart, hapus instance dan device
- **Operasi device yang sudah stabil** (minim bug):
 - `device/info` — info perangkat
 - `scanlog/new` — ambil scanlog baru
 - `scanlog/all` — ambil semua scanlog

> **Catatan:** `user/all` (get all user) masih belum stabil. Cara terbaik untuk sinkronisasi data user adalah **export manual dari mesin absensi**, lalu import langsung ke database.

Endpoint lain yang terdaftar di API belum menjadi prioritas pengembangan dan mungkin belum stabil.

---

## Daftar Isi

- [Perangkat yang Didukung](#perangkat-yang-didukung)
- [Prasyarat](#prasyarat)
- [Device.ini (WAJIB)](#deviceini-wajib)
- [Instalasi & Build](#instalasi--build)
- [Konfigurasi](#konfigurasi)
- [Menjalankan](#menjalankan)
- [Web Dashboard](#web-dashboard)
- [REST API](#rest-api)
- [Arsitektur](#arsitektur)
- [Lisensi](#lisensi)

---

## Perangkat yang Didukung

Gateway ini secara teori mendukung **semua perangkat yang kompatibel dengan Fingerspot EasyLink SDK**. Berikut perangkat yang sudah diuji:

| Model            | Tipe                   | Jumlah Diuji | Status |
|------------------|------------------------|-------------|--------|
| Revo WDV-204BNC  | Fingerprint + Card     | 3 unit      | Teruji |
| Revo WF-206BNC   | Fingerprint + Face ID  | 4 unit      | Teruji |

> Perangkat lain yang menggunakan protokol EasyLink SDK seharusnya kompatibel, namun belum diuji.

---

## Prasyarat

- **Windows** (gateway ini hanya berjalan di Windows karena bergantung pada `FService.exe`)
- **Go 1.25+** (untuk build)
- **EasyLink SDK** — folder `core/` berisi `FService.exe`, DLL, dan OCX

---

## Device.ini (WAJIB)

File `Device.ini` **wajib diisi** sebelum menjalankan gateway. File ini berisi konfigurasi setiap perangkat absensi yang akan dikelola.

Letakkan `Device.ini` di root project (sama dengan lokasi `gateway.exe`).

### Format Device.ini

```ini
[Nama Perangkat]
sn=SERIAL_NUMBER
aktivasi=KODE_AKTIVASI
password=PASSWORD_DEVICE
ip_address=ALAMAT_IP
ethernet_port=PORT_ETHERNET
```

### Contoh

```ini
[Revo WDV-204BNC - Nama Lokasi]
sn=XXXXXXXXXXXXXXX
aktivasi=XXXXX-XXXX-XXXXX-XXXX-XXXXX-XXXX-XXXXX::3-29
password=0
ip_address=192.168.1.100
ethernet_port=5005

[Revo WF-206BNC - Nama Lokasi]
sn=XXXXXXXXXXXXXXXXXXXXX
aktivasi=XXXXX-XXXX-XXXXX-XXXX-XXXXX-XXXX-XXXXX::3-26
password=0
ip_address=192.168.1.101
ethernet_port=5005
```

### Penjelasan Field

| Field           | Wajib | Keterangan |
|-----------------|-------|------------|
| `[Nama Device]` | Ya    | Nama section (bebas, untuk identifikasi) |
| `sn`            | Ya    | Serial number perangkat |
| `aktivasi`      | Ya    | Kode aktivasi dari Fingerspot |
| `password`      | Ya    | Password device (nilai `0` jika tidak ada) |
| `ip_address`    | Ya    | Alamat IP perangkat di jaringan |
| `ethernet_port` | Ya    | Port ethernet perangkat (default `5005`) |

> **Watermark aktivasi:** Kode aktivasi memiliki format `XXXXX-XXXX-XXXXX-XXXX-XXXXX-XXXX-XXXXX::A-B` di mana `A` adalah **watermark opsi** dan `B` adalah **tipe produk** (contoh: `::3-29` untuk tipe 3 opsi 29).

> **Cara mendapatkan `ip_address`, `port`, dan `password`:** Baca panduan di `EasyLink SDK User Guide.pdf`. Ketiga field ini harus diambil langsung dari konfigurasi mesin absensi.

---

## Instalasi & Build

```powershell
# 1. Clone repository
git clone <repo-url>
cd Easylink

# 2. Pastikan folder core/ ada (berisi SDK EasyLink)

# 3. Jalankan build script
.\build.ps1

# Output: gateway.exe di root project
```

> **Catatan:** Folder `core/` tidak termasuk dalam repository. Salin dari instalasi EasyLink SDK yang sudah ada.

---

## Konfigurasi

Konfigurasi gateway dapat diatur melalui file `config.json` atau environment variables.

### config.json (opsional)

```json
{
 "core_path": ".\\core",
 "instances_path": ".\\instances",
 "db_path": ".\\easylink.db",
 "absen_db_path": ".\\absen.db",
 "root_device_ini_path": ".\\Device.ini",
 "gateway_port": 7100,
 "fservice_start_port": 7110,
 "watchdog_interval": "10s"
}
```

### Environment Variables

| Variable                     | Default                     | Keterangan |
|------------------------------|-----------------------------|------------|
| `EASYLINK_CONFIG`            | `config.json`               | Path ke file konfigurasi |
| `EASYLINK_CORE_PATH`         | `.\core`                    | Folder SDK EasyLink |
| `EASYLINK_INSTANCES_PATH`    | `.\instances`               | Folder instance FService |
| `EASYLINK_DB_PATH`           | `.\easylink.db`             | Database utama |
| `EASYLINK_ABSEN_DB_PATH`     | `.\absen.db`                | Database absensi |
| `EASYLINK_ROOT_DEVICE_INI_PATH` | `.\Device.ini`           | File konfigurasi device |
| `EASYLINK_GATEWAY_PORT`      | `7100`                      | Port web gateway |
| `EASYLINK_FSERVICE_START_PORT` | `7110`                    | Port awal untuk FService instances |
| `EASYLINK_WATCHDOG_INTERVAL` | `10s`                       | Interval health check |

---

## Menjalankan

```powershell
# Jalankan gateway
.\gateway.exe

# Buka dashboard di browser
# http://localhost:7100
```

Saat pertama kali dijalankan:
1. Gateway akan membaca `Device.ini` dan menyinkronkan daftar perangkat ke database
2. Database `easylink.db` dan `absen.db` akan dibuat otomatis
3. Instance `FService.exe` akan otomatis di-start jika ada device yang enabled

---

## Web Dashboard

Dashboard tersedia di `http://localhost:7100` dengan halaman-halaman berikut:

| Halaman     | URL              | Fungsi |
|-------------|------------------|--------|
| Dashboard   | `/`              | Ringkasan status sistem |
| Instances   | `/instances`     | Manajemen instance FService (create/start/stop/restart) |
| Devices     | `/devices`       | Daftar & manajemen perangkat |
| Device Detail | `/devices/{id}` | Detail & operasi per device |
| Scanlog     | `/scanlog`       | Log absensi (scanlog) |
| Users       | `/users`         | Manajemen user/fingerprint |
| Test        | `/test`          | Testing koneksi device |
| Jobs        | `/jobs`          | Riwayat job/request ke device |
| Logs        | `/logs`          | Log aktivitas sistem |
| Settings    | `/settings`      | Konfigurasi SetDef & auto-start |

---

## REST API

### Instance Management

| Method   | Endpoint                      | Keterangan |
|----------|-------------------------------|------------|
| `GET`    | `/api/instances`              | List semua instance |
| `POST`   | `/api/instances`              | Buat instance baru |
| `POST`   | `/api/instances/{id}/start`   | Start instance |
| `POST`   | `/api/instances/{id}/stop`    | Stop instance |
| `POST`   | `/api/instances/{id}/restart` | Restart instance |
| `DELETE` | `/api/instances/{id}`         | Hapus instance |

### Device Management

| Method   | Endpoint                         | Keterangan |
|----------|----------------------------------|------------|
| `GET`    | `/api/devices`                   | List semua device |
| `POST`   | `/api/devices`                   | Tambah device baru |
| `GET`    | `/api/devices/{id}`              | Detail device |
| `PUT`    | `/api/devices/{id}`              | Update device |
| `POST`   | `/api/devices/{id}/toggle`       | Enable/disable device |
| `DELETE` | `/api/devices/{id}`              | Hapus device |
| `GET`    | `/api/devices/{id}/config`       | Baca konfigurasi device |
| `PUT`    | `/api/devices/{id}/config`       | Update konfigurasi device |
| `DELETE` | `/api/devices/{id}/config/{key}` | Hapus item konfigurasi |

### Operasi Device

| Method   | Endpoint                         | Keterangan |
|----------|----------------------------------|------------|
| `GET`    | `/api/devices/{sn}/info`         | Info perangkat |
| `GET`    | `/api/devices/{sn}/scan/new`     | Scanlog baru dari device |
| `GET`    | `/api/devices/{sn}/scan/all`     | Semua scanlog dari device |
| `POST`   | `/api/devices/{sn}/scan/delete`  | Hapus scanlog di device |
| `GET`    | `/api/devices/{sn}/scan/gps`     | Data GPS scanlog |
| `GET`    | `/api/devices/{sn}/users`        | List user di device |
| `POST`   | `/api/devices/{sn}/users`        | Set user tunggal |
| `POST`   | `/api/devices/{sn}/users/batch`  | Set user massal |
| `DELETE` | `/api/devices/{sn}/users/{pin}`  | Hapus user |
| `DELETE` | `/api/devices/{sn}/users`        | Hapus semua user |
| `POST`   | `/api/devices/{sn}/time`         | Set waktu device |
| `POST`   | `/api/devices/{sn}/init`         | Inisialisasi device |
| `POST`   | `/api/devices/{sn}/deladmin`     | Hapus admin |
| `POST`   | `/api/devices/{sn}/log/del`      | Hapus log device |

### Sinkronisasi Absensi (Absen DB)

| Method   | Endpoint                                | Keterangan |
|----------|-----------------------------------------|------------|
| `GET`    | `/api/devices/{sn}/scan/logs`           | Scanlog dari absen.db |
| `GET`    | `/api/devices/{sn}/scan/smart`          | Smart fetch scanlog (incremental) |
| `POST`   | `/api/devices/{sn}/scan/sync`           | Trigger sinkronisasi scanlog |
| `POST`   | `/api/devices/{sn}/users/sync`          | Sinkronisasi data user |
| `GET`    | `/api/devices/{sn}/users/{pin}/templates` | Template sidik jari user |
| `GET`    | `/api/devices/{sn}/absen/info`          | Info absensi device |
| `GET`    | `/api/devices/{sn}/absen/compare`       | Bandingkan data absensi |
| `GET`    | `/api/devices/{sn}/absen/users`         | List user dari absen.db |

### Konfigurasi & Monitoring

| Method | Endpoint                  | Keterangan |
|--------|---------------------------|------------|
| `GET`  | `/health`                 | Health check semua instance & device |
| `GET`  | `/api/events`             | SSE stream event/aktivitas (real-time) |
| `GET`  | `/api/jobs`               | Riwayat job request |
| `GET`  | `/api/logs`               | Riwayat log berdasarkan tanggal |
| `GET`  | `/api/config`             | Baca konfigurasi gateway |
| `PUT`  | `/api/config`             | Update konfigurasi gateway |
| `GET`  | `/api/setdef`             | Baca isi SetDef.fin |
| `PUT`  | `/api/setdef`             | Update SetDef.fin & inject ke semua instance |
| `GET`  | `/api/config/auto-start`  | Baca konfigurasi auto-start |
| `PUT`  | `/api/config/auto-start`  | Update konfigurasi auto-start |
| `POST` | `/api/test/device-info`   | Test koneksi ke device |

---

## Arsitektur

```
Browser / API Client
        │
        ▼
┌─────────────────┐
│  EasyLink Gateway│  ← Port 7100 (Web UI + REST API)
│  ┌─────────────┐ │
│  │ Sync Service│ │  ← Sinkronisasi Device.ini → database
│  │ Watchdog    │ │  ← Health check setiap 10s + auto-recovery
│  │ Queue Mgr   │ │  ← Antrian request per-device (serial)
│  │ Syncer      │ │  ← Sinkronisasi scanlog otomatis
│  │ EventLogger │ │  ← Ring buffer + SSE streaming
│  └─────────────┘ │
└────────┬────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌────────┐ ┌────────┐     ┌────────┐
│FService│ │FService│ ... │FService│  ← 1 instance per slot
│ :7110  │ │ :7111  │     │ :711N  │     (port dimulai dari 7110)
└────┬───┘ └────┬───┘     └────┬───┘
    │          │               │
    ▼          ▼               ▼
┌────────┐ ┌────────┐     ┌────────┐
│ Device │ │ Device │ ... │ Device │  ← Revo WDV-204BNC / WF-206BNC
│   A    │ │   B    │     │   N    │     (Ethernet TCP/IP)
└────────┘ └────────┘     └────────┘
```

### Komponen Utama

| Komponen       | Fungsi |
|----------------|--------|
| **SdkManager** | Lifecycle management instance FService.exe (create/start/stop/restart/delete) |
| **Watchdog**   | Monitoring kesehatan instance dan device, auto-restart jika gagal |
| **QueueManager** | Antrian request serial per-device untuk mencegah collision |
| **Syncer**     | Sinkronisasi scanlog otomatis dari device ke `absen.db` |
| **SyncService** | Sinkronisasi `Device.ini` ke database |
| **EventLogger** | Log aktivitas real-time via SSE (Server-Sent Events) |

---

## Lisensi

Proprietary — Menggunakan Fingerspot EasyLink SDK.

© 2026
