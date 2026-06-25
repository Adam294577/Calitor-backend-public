# 退貨單金額符號規範:讀取端 abs × sign

Calitor 出貨 / 退貨單(`shipments` 表)在 DB 中**正負儲存不對稱**,讀取端必須統一用「`math.Abs(value) × sign`」處理。

## 現況(DB 端的不對稱)

寫入路徑(`CreateShipment` / `UpdateShipment`,在 `controllers/shipment_controller.go`)對退貨單(`shipment_mode = 4`)的處理:

| 欄位 | 寫入時是否翻負 |
|---|---|
| `deal_amount` | ✅ 有翻負(`if req.ShipmentMode == 4 { dealAmount = -dealAmount }`,Create 第 498、Update 第 798 行) |
| `tax_amount` | ❌ 不翻負(直接 `tax_amount: req.TaxAmount`) |
| `discount_amount` | ❌ 不翻負(直接 `discount_amount: req.DiscountAmount`) |
| `invoice_amount` | ❌ 不翻負 |
| `charge_amount` | 由 Gather 沖銷回寫,另一條路徑 |

因此一張 RYY 退貨單在 DB 中典型樣貌:`deal_amount = -500`、`tax_amount = +24`、`discount_amount = +0`。

加上歷史資料來源混雜(舊系統 / 條碼批次 / 手動修補),同一欄位在不同列可能出現正、負、或 0 — 因此**不能假設「退貨欄位永遠是負」也不能假設「永遠是正」**。

## 新規範(2026-05-28 起):寫入端也對稱

從 `spec/0528/退貨計算異常/` 起,**寫入端**(`CreateShipment` / `UpdateShipment`)對 `shipment_mode = 4` 的退貨單,`deal_amount` / `tax_amount` / `discount_amount` / `invoice_amount` 一律 `math.Abs(value) × sign` 翻負儲存,讓 DB 上是對稱有號值。

從此「前端 save 後 DB 就是正常值」,不再是「DB 錯但讀取時翻號顯示對」。

但**讀取端的 abs × sign 防呆繼續保留**,理由:
- 歷史 RYY 資料(2026 修正前)有混合 — 5,811 筆 tax 是負(舊系統 / 早期寫入路徑)、5 筆 tax 是正(2026 寫入錯誤)、9,380 筆零
- 不做 batch migration,歷史資料逐筆編輯逐筆修正
- abs × sign 對 +/-/0 三狀態冪等正確,對新舊資料都不會弄錯

未來其他寫入路徑(如條碼批次匯入 `shipment_batch_controller`,若有寫 tax/discount)也應比照此規範。

## 慣例(讀取端)

**所有讀取退貨單金額的計算,先 `math.Abs()` 取絕對值,再依 `shipment_mode` 翻號:**

```go
sign := float64(1)
if s.ShipmentMode == 4 {
    sign = -1
}
amount := math.Abs(s.SomeAmount) * sign
```

SQL 端對應寫法(已用於 `shipment_summary` 的 CTE):

```sql
ABS(some_amount) * (CASE WHEN shipments.shipment_mode = 4 THEN -1 ELSE 1 END)
```

**Why:** 註解見 `controllers/shipment_summary_controller.go:329`「符號規範:出貨一律正、退貨一律負,避免歷史資料正負不一造成『負負得正』」與第 449 行 SQL CTE 註解「ABS() 對齊原本 Go 的 math.Abs() 防呆,歷史 mode=4 存負值不會『負負得正』」。

## 已套用此 pattern 的位置

- `controllers/shipment_summary_controller.go` — 第 328–360 行(detail)、第 443 行(SQL `signCase`)
- `controllers/retail_sell_controller.go` — 第 250–294、509–560 行(零售 qty / price / cash / card / gift)
- `controllers/shipment_batch_controller.go` — 第 136、292、395 行(條碼批次)
- `controllers/receivable_controller.go` — 第 217–227 行(應收帳款查詢 trade / tax / discount,2026-05-28 補上,即 `spec/0528/退貨計算異常/` 修補)
- `controllers/shipment_controller.go::CreateShipment` — 第 484–520 行附近(寫入端對 RYY 翻負 deal/tax/discount/invoice,2026-05-28 新增)
- `controllers/shipment_controller.go::UpdateShipment` — 第 784–820 行附近(同 Create,寫入端對 RYY 翻負四欄,2026-05-28 新增)

## 反面教材

`receivable_controller.go:218` 修補前(2026-05-28 前):

```go
tradeAmount := receivable.RoundAmount(s.DealAmount - s.TaxAmount + s.DiscountAmount)
```

對 SYY 出貨對(`+500 - 24 + 0 = +476`),對 RYY 退貨錯(`-500 - 24 + 0 = -524`)— 因為 `s.DealAmount` 已翻負但 `s.TaxAmount` 沒翻負,額外多扣一個稅額。

實際 bug 顯示:RYY2026050002 交易金額顯示 -524,應為 -476。

## 未來新模組的取捨建議

進貨模組(`stocks` 表,有 `stock_mode`:1=進貨 / 2=退貨)目前**沒有應付帳款查詢功能**,Stock model 連 `DealAmount` 欄位都沒有(`models/stock.go`)。若未來實作,兩種走法:

### 走法 A(建議,但前提是該模組是全新功能、無歷史包袱)
寫入端對 `stock_mode = 2` 退貨時,**所有金額欄位**(deal、tax、discount、invoice)一律翻負儲存。
讀取端公式直接套 `deal - tax + discount` 即可,**無需 abs**。
優點:DB 帶號,語意清晰。
適用:全新欄位 / 全新功能,沒有任何歷史資料。

### 走法 B(沿用既有慣例)
寫入端只翻 deal(像 shipment 一樣),其他不翻;讀取端統一套 `math.Abs(value) × sign`。
優點:跟既有 shipment 慣例一致;若歷史資料正負已混雜,abs 防呆有效。
缺點:多一層讀取轉換;每個讀取點都要套,容易漏。

**判斷標準:有歷史包袱選 B、純新功能選 A,且決定後在新模組的 controller 加註明示走哪一條,避免下一位開發者再陷入「為何 deal 是負但 tax 是正」的疑惑。**

## How to apply

1. 任何新增讀取 `shipments` 表金額的 controller / SQL,默認套 abs × sign,除非確認該欄位永遠帶號
2. Code review 時遇到 `s.SomeAmount` 直接運算的位置,問一句「對 RYY 是否正確」
3. 新增前端報表 / 列印模板,若數字會出現「合計對不起來」「退貨那行多扣一個稅」等症狀,優先檢查讀取端是否漏了 abs × sign
