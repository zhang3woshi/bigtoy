# BigToy Garage

BigToy is a die-cast model gallery + admin backend.

[English](./README.md) | [简体中文](./README.zh-CN.md)

## 中文文档

- 简体中文版说明： [README.zh-CN.md](./README.zh-CN.md)

## Homepage Screenshots

### Desktop

![Homepage desktop screenshot](./docs/screenshots/home-desktop.png)

### Tablet

![Homepage tablet screenshot](./docs/screenshots/home-tablet.png)

### Mobile

![Homepage mobile screenshot](./docs/screenshots/home-mobile.png)

## Stack

- Backend: Go + Beego v2
- Frontend: Vite + Vue 3
- Database: SQLite (`backend/data/models.db`)
- Images: `backend/data/images/<model-id>/`

## Features

- Public gallery page (`/index.html`)
- Model detail page (`/model.html`)
- Admin page (`/admin.html`) with authentication
- Create / update / delete models (admin only)

## Data Paths

- SQLite DB: `backend/data/models.db`
- Image root: `backend/data/images`
- Public image URL mapping: `/uploads/<id>/<file>`

## Automatic Backup

Backend now includes a scheduled backup job:

- Source: `backend/data/models.db` + `backend/data/images/`
- Output: `backend/data/backup/backup_*.zip`
- Retention: keep latest 3 backups, delete the oldest when exceeded

Config (in `backend/conf/app.conf` or env vars):

```ini
backup_enabled = true
backup_interval_minutes = 1440
backup_max_files = 3
```

Environment variable equivalents:

- `BIGTOY_BACKUP_ENABLED`
- `BIGTOY_BACKUP_INTERVAL_MINUTES`
- `BIGTOY_BACKUP_MAX_FILES`

## Run Locally

### Backend

```powershell
cd backend
go mod tidy
go run main.go
```

Backend default: `http://localhost:8080`

### Frontend dev

```powershell
cd frontend
npm.cmd install
npm.cmd run dev
```

Frontend default: `http://localhost:5173`

## Unit Tests And Coverage

### Backend (Go)

Run backend unit tests:

```powershell
cd backend
go test ./...
```

Run backend service-layer coverage (threshold: >= 85%):

```powershell
cd backend
powershell -ExecutionPolicy Bypass -File .\check-coverage.ps1
```

### Frontend (Vite + Vitest)

Run frontend unit tests:

```powershell
cd frontend
npm.cmd run test
```

Run frontend coverage (threshold: >= 85%):

```powershell
cd frontend
npm.cmd run test:coverage
```

## Build Frontend To Backend Static

Run in project root:

```powershell
powershell -ExecutionPolicy Bypass -File .\sync-frontend-static.ps1
```

This exports frontend assets into `backend/static`.

## GitHub Release Automation

This repository includes a GitHub Actions workflow at `.github/workflows/release.yml`.

- Trigger: push a tag matching `v*` (for example: `v1.0.0`)
- Behavior:
  - Build frontend and export assets to `backend/static`
  - Cross-compile backend for Linux/macOS/Windows (`amd64`)
  - Package runtime files (`conf/`, `static/`, basic `data/` folders, `README`, `LICENSE`)
  - Publish all archives to GitHub Release with auto-generated notes

Example:

```powershell
git tag v1.0.0
git push origin v1.0.0
```

## Initialize Admin Password

Use bcrypt hash in config or environment variables.

### 1) Generate bcrypt hash

```powershell
cd backend
go run ./cmd/hashpass -password "YourStrongPassword"
```

Example output:

```text
$2a$10$xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### 2) Configure admin credentials

Option A: environment variables

```powershell
$env:BIGTOY_ADMIN_USER="admin"
$env:BIGTOY_ADMIN_PASSWORD_HASH="the_hash_from_previous_step"
```

Option B: `backend/conf/app.conf`

```ini
admin_user = admin
admin_password_hash = the_hash_from_previous_step
```

### 3) Production recommendation

- Use non-dev run mode
- Enable secure cookie under HTTPS

```ini
auth_secure_cookie = true
```

### 4) Load sensitive values from local file (recommended for open-source repos)

1. Copy template:

```powershell
cd backend
Copy-Item .\conf\secrets.env.example .\conf\secrets.env
```

2. Edit `conf/secrets.env` and fill real values.

3. Start backend normally:

```powershell
go run main.go
```

The program auto-loads `conf/secrets.env` on startup. This file is git-ignored.

You can also specify a custom secrets file path:

```powershell
$env:BIGTOY_SECRETS_FILE="C:\secure\bigtoy.secrets.env"
```
## Install As System Service (kardianos/service)

The backend executable now supports service lifecycle commands.

### 1) Build executable

```powershell
cd backend
go build -o bigtoy.exe main.go
```

### 2) Install service (run terminal as Administrator)

```powershell
.\bigtoy.exe service install
.\bigtoy.exe service start
```

### 3) Service management

```powershell
.\bigtoy.exe service status
.\bigtoy.exe service restart
.\bigtoy.exe service stop
.\bigtoy.exe service uninstall
```

Notes:

- Keep `bigtoy.exe`, `conf/`, `data/`, and `static/` in expected relative locations.
- Service startup will auto-adjust working directory to the executable folder when needed.

## Public HTTPS Deployment (HTTP Off)

If this service is exposed to the public internet, use HTTPS-only mode.

### Option A: `app.conf` with prod run mode

`backend/conf/app.conf` already contains prod defaults:

```ini
prod::force_https = true
prod::enablehttp = false
prod::enablehttps = true
prod::httpsaddr = 0.0.0.0
prod::httpsport = 443
prod::https_cert_file = conf/certs/fullchain.pem
prod::https_key_file = conf/certs/privkey.pem
prod::auth_secure_cookie = true
```

Set:

```ini
runmode = prod
```

Then replace certificate paths with your real files.

### Option B: environment variables (service friendly)

```powershell
$env:BIGTOY_FORCE_HTTPS="true"
$env:BIGTOY_ENABLE_HTTP="false"
$env:BIGTOY_ENABLE_HTTPS="true"
$env:BIGTOY_HTTPS_PORT="443"
$env:BIGTOY_HTTPS_CERT_FILE="C:\path\to\fullchain.pem"
$env:BIGTOY_HTTPS_KEY_FILE="C:\path\to\privkey.pem"
$env:BIGTOY_AUTH_SECURE_COOKIE="true"
```

Important:

- When HTTPS is enabled, cert and key files are required.
- If HTTP and HTTPS are both disabled, startup will fail.
- Set `BIGTOY_ALLOWED_ORIGINS` (or `allowed_origins`) to your public origin, for example:
  - `https://your-domain.com`

## API

- `GET /api/models`
- `POST /api/models` (auth required)
- `PUT /api/models/:id` (auth required)
- `DELETE /api/models/:id` (auth required)
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`

