# 庫點 / 分店識別:雙軌字段 + store_lookup 反查

Calitor 系統「庫點 / 分店」識別字段是雙軌的,**報表類功能不能只看其中一邊**:

- `retail_sells.sell_store` 存 **`customer.code`**(如 `'01'`、`'0SH01'`)
- `shipments.ship_store` 存 **`customer.branch_code`**(如 `'YY'`、`'Z1'`)
- `product_size_stocks.customer_id` 直接 FK 到 `retail_customers.id`
- 後端 `stock_adjust` 從銷售觸發時用 `WHERE code = sell_store` 反查客戶,從出貨觸發時用 `WHERE branch_code = ship_store` 反查;同一家店因此可能在三個來源用不同字串/不同 customer_id 出現

**Why:** 早期實作把 `branch_code` 當庫點代號塞在出貨流程,後續又改用 `code` 在銷售流程,歷史包袱清不掉 — 同一家「尚宇 01 店」在銷售紀錄是 `'01'`、出貨紀錄是 `'YY'`、stocks 表又另開一筆 customer。直接以 `customer_id` 或 `branch_code` 分組會把同一家店拆成多列,與使用者認知不符。

## How to apply

### 1. 任何「依分店」報表必走 store_lookup OR 反查

把 raw store_code 統一回 canonical `customer.code` 才能正確合併:

```sql
LEFT JOIN LATERAL (
    SELECT code, short_name, name FROM retail_customers
    WHERE deleted_at IS NULL
      AND (code = ds.raw_store OR branch_code = ds.raw_store)
    ORDER BY (CASE WHEN code = ds.raw_store THEN 0 ELSE 1 END), id
    LIMIT 1
) rc ON TRUE
```

`ORDER BY` 的 `CASE` 保證 code 優先 — 同字串同時匹配 code 和某客戶 branch_code 時,挑 code 那個。

### 2. 顯示一律用 canonical `customer.code`

**不要顯示 `branch_code`**。前端下拉 / Excel 匯出 / 表格欄位都一律 code。

### 3. branch_ids 過濾要雙條件 OR

`branch_ids`(`customer.id` 列表)過濾時,需把選到的客戶的 `code` 與 `branch_code` 都納入 `sell_store` / `ship_store` IN 比對範圍(雙條件 OR),參考 `product_in_out_summary_controller.go` 的 `buildBranchSellFilter` 樣式。

## 已實作的範本

- `product_sales_stats_controller.go`「依分店」分組(最早採用 store_lookup CTE)
- `product_sales_summary_controller.go` `group_by_branch=1`(2026-05-15 重構,跟 stats 對齊)
- `product_in_out_summary_controller.go` 雙條件 branch 過濾
