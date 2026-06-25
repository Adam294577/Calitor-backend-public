# Calitor Backend

服飾／零售業 ERP 後台的後端 API。Go + Gin + GORM + PostgreSQL，前後端分離，前端為獨立 Vue SPA。

> 本 repo 為作品集公開版本，已移除所有正式環境的設定與密鑰，請依下方說明複製 `*.example` 自行配置。

## 技術棧

- **語言 / 框架**：Go 1.25、Gin
- **ORM / DB**：GORM、PostgreSQL（讀寫分離 Master/Slave）
- **快取 / Rate-limit**：Redis（go-redis v9）
- **認證**：JWT（權限資訊內嵌 token）
- **物件儲存**：MinIO / GCS（商品圖片上傳代理）
- **設定**：Viper（依環境載入 YAML，可由環境變數覆蓋）
- **排程**：robfig/cron（清理孤兒圖片等）
- **API 文件**：Swaggo

## 主要模組

- 認證授權：登入、RBAC 角色權限（page／func 節點分流）、帳號多角色、登入 rate limit（帳號＋IP）、IP 白名單防火牆 middleware
- 主檔管理：商品（多廠商價格、5 張類別表）、客戶、廠商、幣別匯率、銀行帳號、材質／尺寸群組
- 進銷存：訂貨、採購、進貨、出貨、庫存調撥、零售／店櫃銷售，連動即時庫存、信用額度檢查與含稅計算
- 條碼流程：TXT 條碼解析、多客戶／多庫點批次匯入建單
- 應收帳款：應收查詢、收款沖銷、預收折讓、帳齡分析
- 營運報表：多種統計與 Excel 匯出

## 環境需求

- Go 1.25+
- PostgreSQL、Redis、MinIO（或相容的 S3 物件儲存）

## 設定與啟動

```bash
# 1. 複製設定範本並填入實際值（這些檔已被 .gitignore）
cp config/config_prod.yaml.example config/config_prod.yaml
cp config/config_dev.yaml.example config/config_dev.yaml
cp .env.example .env          # 容器部署亦可只用環境變數

# 2. 安裝依賴
go mod download

# 3. 啟動（預設讀 config/config_prod.yaml；指定環境可用 ENV 或 CLI 參數）
ENV=dev go run .
#   或
go run . dev

# 首次建立資料表
RUN_MIGRATE=true ENV=dev go run .
```

## 🚀 Demo 一鍵啟動（給審閱者）

### 方式 A：用內附 docker-compose 起依賴（最省事，推薦）

`docker-compose.yml` 已備好 PostgreSQL / Redis / MinIO，且 `config_dev.yaml.example` 預設值已對應它。

**macOS / Linux（bash）**

```bash
docker compose up -d                                       # 起 PG/Redis/MinIO
cp config/config_dev.yaml.example config/config_dev.yaml   # 預設值即可直接用
RUN_MIGRATE=true DEMO_MODE=true go run . dev               # 建表 + seed 帳號/權限 + demo 業務資料
```

**Windows（PowerShell）** — 不能用 `VAR=value cmd`，要先 `$env:` 設變數：

```powershell
docker compose up -d
Copy-Item config\config_dev.yaml.example config\config_dev.yaml
$env:RUN_MIGRATE="true"; $env:DEMO_MODE="true"; go run . dev
```

> 第二次之後啟動（資料已在，不用再建表/灌資料）：
> - bash：`DEMO_MODE=true go run . dev`
> - PowerShell：先 `Remove-Item Env:RUN_MIGRATE`（清掉前次設定），再 `$env:DEMO_MODE="true"; go run . dev`
>
> 收掉依賴：`docker compose down`（保留資料）或 `docker compose down -v`（連資料一起清）。

### 方式 B：接你自己的 PG/Redis/MinIO（純環境變數）

```bash
# bash
cp .env.example .env          # 填入你的連線資訊；已預設 DEMO_MODE/SEED_DEMO/admin 密碼
RUN_MIGRATE=true go run .      # ENV=prod 找不到 yaml 時改用 .env
```

```powershell
# PowerShell
Copy-Item .env.example .env
$env:RUN_MIGRATE="true"; go run .
```

啟動後以前端登入：

- 帳號：`admin`
- 密碼：`.env` 的 `SERVER_SEEDADMINPASSWORD`（預設 `demo1234`）

即可看到已有資料的商品、客戶、廠商、進銷存單據與營運報表（demo 資料含商品 25、廠商/客戶各 10、進貨 8、銷售 10、訂貨 6，數字自洽）。

> **Demo 開關（正式環境務必關閉）**
> - `DEMO_MODE=true`：跳過 IP 白名單防火牆，任何來源都能存取。
> - `SEED_DEMO=true`：`RUN_MIGRATE=true` 時一併灌入 demo 業務假資料（以商品表是否已有資料做冪等，重複啟動不會重灌）。
> - MinIO 僅商品圖片上傳會用到；demo 商品圖留空，不上傳圖片也能正常瀏覽。

## 設定載入規則

`config/config_<ENV>.yaml` 由 Viper 依 `ENV` 環境變數或 CLI 第一個參數載入（預設 `prod`）。
任一設定可用環境變數覆蓋，規則為將 key 的點號換成底線並大寫，例如 `Redis.Host` → `REDIS_HOST`。
找不到實體 yaml 時會完全改用環境變數（適合容器部署）。

## Docker

```bash
docker build -t calitor-backend .
docker run -p 8002:8002 --env-file .env calitor-backend
```
