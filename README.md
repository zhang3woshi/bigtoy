# BigToy Garage

BigToy 是一个车模收藏展示站点 + 管理后台项目，当前实现基于 Go(Beego) + Vue 3。

## 页面截图

### Desktop

![Homepage desktop screenshot](./docs/screenshots/home-desktop.png)

### Tablet

![Homepage tablet screenshot](./docs/screenshots/home-tablet.png)

### Mobile

![Homepage mobile screenshot](./docs/screenshots/home-mobile.png)

## 当前页面与功能

| 路由 | 页面 | 访问控制 | 主要功能 |
| --- | --- | --- | --- |
| `/` / `/index.html` | 前台首页 | 公开 | 轮播主视觉、收藏统计、最新收藏、分类浏览、收藏列表搜索/筛选/排序、详情弹窗 |
| `/model` / `/model.html?id=<uuid>` | 详情页 | 公开 | 按车型 ID 展示单个车型详情 |
| `/login` / `/login.html` | 登录页 | 公开 | 管理员账号密码登录，已登录会跳转后台 |
| `/admin` / `/admin.html` | 管理后台（新增+列表） | 需登录 | 新增车型、上传主图/附图、车型列表分页+排序、编辑跳转、删除、备份导入导出 |
| `/admin-edit` / `/admin-edit.html?id=<uuid>` | 管理后台（编辑） | 需登录 | 编辑车型字段、预览现有主图与附图、替换图片并保存 |

## 核心能力

- 前台展示：
  - 首页支持关键词搜索（名称/编号/系列/标签）和品牌筛选。
  - 收藏列表支持按“加入时间/编号”排序，并可升降序切换。
  - 支持卡片详情弹窗与独立详情页两种查看方式。
- 后台管理：
  - 基于 Cookie 会话鉴权，未登录访问后台会自动跳转登录页。
  - 支持新增、编辑、删除车型。
  - 新增/编辑支持 `multipart/form-data` 上传主图(`imageFile`)和多张附图(`galleryFiles`)。
  - 车型列表支持分页（每页 10 条）和排序。
  - 支持 ZIP 备份导出与导入覆盖恢复。
- 数据与安全：
  - 数据库存储为 SQLite：`backend/data/models.db`。
  - 图片存储目录：`backend/data/images/<model-id>/`，对外路径映射 `/uploads/<id>/<file>`。
  - 模型主键使用 UUID，兼容老数据导入并可迁移旧整型 ID。
  - 登录失败支持限流锁定（默认 5 次失败后锁 15 分钟）。

## 技术栈

- Backend: Go 1.24 + Beego v2 + SQLite(modernc)
- Frontend: Vite 7 + Vue 3
- Test: Go test + Vitest

## API（当前实现）

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/models` | 否 | 获取全部车型（返回 `data` + `total`） |
| `POST` | `/api/models` | 是 | 新增车型（支持 JSON / multipart） |
| `PUT` | `/api/models/:id` | 是 | 更新车型（支持 JSON / multipart） |
| `DELETE` | `/api/models/:id` | 是 | 删除车型并清理对应图片目录 |
| `POST` | `/api/auth/login` | 否 | 登录，写入 HttpOnly Cookie |
| `POST` | `/api/auth/logout` | 否 | 登出并清理 Cookie |
| `GET` | `/api/auth/me` | 否 | 查询当前登录态 |
| `GET` | `/api/backup/export` | 是 | 导出 ZIP 备份（数据库 + 图片） |
| `POST` | `/api/backup/import` | 是 | 导入 ZIP 并覆盖恢复数据 |

车型写入字段（新增/编辑）：

- 基础字段：`name`(必填), `modelCode`, `brand`, `series`, `scale`, `year`, `color`, `material`, `condition`, `notes`, `tags`
- 图片字段：`imageUrl`, `gallery`（JSON 模式）或 `imageFile`/`galleryFiles`（multipart 上传模式）
- 上传限制：请求体解析上限 64MB；单张图片文件上限 20MB；备份导入 ZIP 上限 1GB

## 本地启动

### 1) 启动后端

```powershell
cd backend
go mod tidy
go run main.go
```

默认地址：`http://localhost:11000`

### 2) 配置管理员账号（推荐）

生成密码哈希：

```powershell
cd backend
go run ./cmd/hashpass -password "YourStrongPassword"
```

把输出哈希写入环境变量或 `backend/conf/app.conf`/`backend/conf/secrets.env`：

- `BIGTOY_ADMIN_USER`
- `BIGTOY_ADMIN_PASSWORD_HASH`

说明：

- `dev` 模式下如果没配置 `BIGTOY_ADMIN_PASSWORD_HASH`，系统会在日志打印一次性临时密码。
- 非 `dev` 模式下未配置该哈希会启动失败。

### 3) 启动前端开发服务器

```powershell
cd frontend
Copy-Item .env.example .env.local
```

将 `frontend/.env.local` 改为：

```env
VITE_API_BASE=http://localhost:11000
```

然后启动：

```powershell
npm.cmd install
npm.cmd run dev
```

默认前端地址：`http://localhost:5173`

注意：前端开发默认 API 基地址是 `http://localhost:8080`，如果后端用默认 `11000` 端口，需要按上面配置 `VITE_API_BASE`。

### 4) 导出前端静态资源到后端

在项目根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\sync-frontend-static.ps1
```

会把前端产物输出到 `backend/static`，由后端直接提供页面与静态资源。

## 自动备份

后端启动后可按配置定时生成备份 ZIP：

- 来源：`backend/data/models.db` + `backend/data/images/`
- 输出：`backend/data/backup/backup_*.zip`
- 保留策略：默认仅保留最新 3 份

配置项（`backend/conf/app.conf` 或同名环境变量）：

```ini
backup_enabled = true
backup_interval_minutes = 1440
backup_max_files = 3
```

对应环境变量：

- `BIGTOY_BACKUP_ENABLED`
- `BIGTOY_BACKUP_INTERVAL_MINUTES`
- `BIGTOY_BACKUP_MAX_FILES`

## 配置与部署要点

- 机密配置加载：
  - 程序启动会自动读取 `backend/conf/secrets.env`（文件已在 `.gitignore`）。
  - 可通过 `BIGTOY_SECRETS_FILE` 指定自定义路径。
- HTTPS / 端口 / 跨域：
  - 支持 `BIGTOY_ENABLE_HTTP`、`BIGTOY_ENABLE_HTTPS`、`BIGTOY_HTTP_PORT`、`BIGTOY_HTTPS_PORT` 等配置。
  - 生产推荐开启 HTTPS，并配置 `BIGTOY_HTTPS_CERT_FILE` / `BIGTOY_HTTPS_KEY_FILE`。
  - 跨域白名单可通过 `BIGTOY_ALLOWED_ORIGINS` 配置（逗号分隔）。

## 以系统服务运行（kardianos/service）

```powershell
cd backend
go build -o bigtoy.exe main.go
.\bigtoy.exe service install
.\bigtoy.exe service start
```

常用管理命令：

```powershell
.\bigtoy.exe service status
.\bigtoy.exe service restart
.\bigtoy.exe service stop
.\bigtoy.exe service uninstall
```

## 测试与覆盖率

### Backend

```powershell
cd backend
go test ./...
powershell -ExecutionPolicy Bypass -File .\check-coverage.ps1
```

`check-coverage.ps1` 默认检查 `services` 包覆盖率不低于 85%。

### Frontend

```powershell
cd frontend
npm.cmd run test
npm.cmd run test:coverage
```

前端覆盖率阈值配置为 85%（当前覆盖统计范围：`src/js/api.js`）。

## GitHub Release 自动化

仓库已提供 `.github/workflows/release.yml`：

- 触发：推送 `v*` 标签（如 `v1.0.0`）
- 动作：
  - 构建前端并同步到 `backend/static`
  - 交叉编译后端（Linux/macOS/Windows, amd64）
  - 打包 `conf/`、`static/`、基础 `data/`、`README`、`LICENSE`
  - 自动发布 GitHub Release

示例：

```powershell
git tag v1.0.0
git push origin v1.0.0
```
