# Calitor Backend - Go + Gin + GORM Agent

## Project Overview
Calitor 後端 API,Go + Gin + GORM + PostgreSQL,前端為獨立 Vue SPA(`佳立特/Calitor-frontend`)。

## Tech Stack

- **Language**: Go 1.25
- **Web Framework**: Gin v1.12
- **ORM**: GORM v1.31(`gorm.io/driver/postgres` v1.6)
- **Database**: PostgreSQL
- **Cache / Rate-limit**: Redis v9(`redis/go-redis/v9`)
- **Auth**: JWT v5(`golang-jwt/jwt/v5`)
- **Object Storage**: MinIO(`minio-go/v7`)+ GCS(`cloud.google.com/go/storage`)
- **Config**: Viper(`spf13/viper`),YAML per env
- **Logging**: Logrus + file-rotatelogs(輪轉)
- **Scheduler**: `robfig/cron/v3`
- **API Docs**: Swaggo
- **Dev Hot-reload**: Air(`cosmtrek/air`)

## Architecture

專案根目錄結構:

- `config/` — 各環境 YAML(`config_dev.yaml`、`config_prod.yaml`),由 Viper 依 CLI 第一個參數或 `ENV` env var 載入,預設 `prod`
- `controllers/` — HTTP request handlers
- `middlewares/` — auth、rate-limit、firewall 等
- `models/` — GORM entities(`product.go`、`retail_customer.go` 等)
- `routes/` — 路由註冊
- `services/` — 業務邏輯子套件:
  - `pricing/` — 含稅/未稅計算(see `non_tax.go` / `non_tax_test.go`)
  - `permission/` — 權限與 master code 處理
  - `stock/`、`inventory/` — 庫存
  - `delivery/`、`purchase/` — 出貨、採購
  - `redis/`、`storage/`、`firewall/`、`barcode/`、`library/`、`log/`、`responses/`、`curl/`、`common/`、`modify/`、`receivable/`
- `cron/` — 排程任務設定
- `tmp/`、`tmp-dev/`、`tmp-prod/` — Air 編譯產物

## Run

### Dev(本機,PowerShell)

每次啟動要先設 `RUN_MIGRATE` 環境變數,決定是否跑 DB migration:

```powershell
# 第一次啟動 / migration 檔有新增時:跑 migration
$env:RUN_MIGRATE="true"; air -c .air.dev.toml

# 平時 reload / 不需要 migration 時:跳過
$env:RUN_MIGRATE="false"; air -c .air.dev.toml
```

Air 監看 `.go`、`.yaml`、`.yml`,變動自動 rebuild。

### Production

Dockerfile two-stage build(alpine runtime),啟動於 port **8002**:

```bash
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main .
./main         # 預設讀 config_prod.yaml
```

### 環境切換

`./main dev` 或 `ENV=dev ./main` → 讀 `config_dev.yaml`。

## Testing

- **框架**:Go stdlib `testing`(**無 testify**),table-driven 為主
- **執行指令**:`go test ./...`
- **既有測試檔**(4 個):
  - `services/permission/master_code_test.go`
  - `services/pricing/non_tax_test.go`
  - `controllers/sort_model_code_test.go`
  - `middlewares/rate_limit_test.go`
- **HTTP 測試模式**:用 `httptest.NewRecorder()` + `gin.CreateTestContext()`
- ⚠️ **無 Makefile、無 `.github/workflows/`、無 `.golangci.yml`** — commit 前自行跑 `go test ./...`,別找 npm run / make test 那種包裝

## Coding Conventions

- 標準 Go gofmt
- Imports 順序:stdlib → 第三方 → 內部包
- Errors:**回傳 `error`**,不要 panic;測試中以 `t.Errorf` / `t.Fatalf` 斷言
- Naming:exported `CamelCase`、unexported `camelCase`(Go 標準)
- Logging:用 logrus 全域 logger,**不要** `fmt.Println`/`log.Print`

## Project Conventions

公司端業務雷區與安全設計方法論,由 Claude 與人類開發者共同遵循。動相關範圍前必讀:

- @docs/conventions/product-brand-dual-fields.md — products 的 `brand_id` / `billing_brand` 雙欄位陷阱(訂貨單對帳品牌過濾失靈的歷史 bug)
- @docs/conventions/store-lookup-pattern.md — `sell_store` / `ship_store` 雙軌 + `store_lookup` 反查 CTE(報表類功能合併同店的關鍵)
- @docs/conventions/security-design-threats.md — 安全設計初版必列威脅情境四類(含 commit `9f5234d` 兩因子 rate-limit 實例)
- @docs/conventions/return-amount-sign-normalization.md — 退貨單(`shipment_mode=4`)讀取時一律 `math.Abs() × sign` 防呆,避免 DB 中 deal_amount 翻負但 tax/discount 未翻造成的偏差

## Notes

- `tmp*/` 是編譯產物,已 `.gitignore`,不要手動編輯
- API 文件由 Swaggo 從 godoc 註解生成,改 handler 註解後需 `swag init`
