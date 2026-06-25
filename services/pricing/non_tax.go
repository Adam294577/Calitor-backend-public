package pricing

import "math"

// ResolveNonTaxPrice 永遠從 displayPrice + TaxMode/TaxRate 反推未稅單價,
// 不再 trust 前端送的 reqNonTax — 避免「前端送 wholesale 快照,後端原樣寫入」
// 導致 non_tax_price 與 order_price/ship_price 公式脫鉤。
// 公式對齊前端 SizeQtyTable.vue 與 utils/shipmentMath.js 的 toDisplayPrice 反函數。
//
// reqNonTax 僅在 displayPrice<=0 的零價穿透(贈品/樣品)場景被回傳,
// 其他情況一律以 displayPrice 為 source of truth。
func ResolveNonTaxPrice(reqNonTax, displayPrice float64, taxMode int, taxRate float64) float64 {
	if displayPrice <= 0 {
		return reqNonTax
	}
	if taxMode == 1 && taxRate > 0 {
		return math.Round(displayPrice / (1 + taxRate/100))
	}
	return displayPrice
}
