# BigToy Garage

BigToy is a die-cast model gallery + admin backend.

## Stack

- Backend: Go + Beego v2
- Frontend: Vite (Vanilla JS)
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

## Build Frontend To Backend Static

Run in project root:

```powershell
powershell -ExecutionPolicy Bypass -File .\sync-frontend-static.ps1
```

This exports frontend assets into `backend/static`.

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

