# 佳立特商品「對帳品牌」雙欄位陷阱

`models/product.go` 上有兩個被註解為「對帳品牌」的欄位:

- `BrandId *int64` (FK → `brands.id`) — 訂貨單 `SearchProducts`、商品列表 `GetProducts` 都用這個 `WHERE brand_id = ?` 過濾
- `BillingBrand string` (varchar(100), 存 `brand.code`) — 商品編輯頁面 (`products.vue` `form.billing_brand`, `:value="b.code"`) **唯一**會寫入的欄位

`CreateProduct` / `UpdateProduct` request 結構**完全沒有** `BrandID`,所以 `brand_id` 在介面流程中永遠不會被更新(維持 NULL 或舊值)。

**Why:** 2026-05-11 用戶回報「訂貨單對帳品牌選 VM 找不到 GB809*」即此 bug;商品主檔顯示 VM 是因為讀的是 `billing_brand`,但訂貨單過濾的是 `brand_id`,兩者從未同步。

**How to apply:** 任何牽涉商品「對帳品牌」過濾或顯示的需求,先確認讀寫的是哪個欄位。修復方向建議:
1. `CreateProduct` / `UpdateProduct` 增加 `brand_id` 欄位
2. 寫一次性 SQL 把 `billing_brand` 對應到 `brand_id` 補齊
3. 商品編輯頁改綁 `brand_id`
4. `billing_brand` 暫時保留向後相容
