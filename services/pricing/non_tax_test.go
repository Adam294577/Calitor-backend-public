package pricing

import "testing"

func TestResolveNonTaxPrice(t *testing.T) {
	cases := []struct {
		name      string
		reqNonTax float64
		displayPx float64
		taxMode   int
		taxRate   float64
		want      float64
	}{
		{
			name:      "應稅 reqNonTax=752 但 displayPx=677 → 強制以公式為準,覆寫為 677(對齊 OYY2605130)",
			reqNonTax: 752, displayPx: 677, taxMode: 2, taxRate: 5,
			want: 677,
		},
		{
			name:      "含稅 reqNonTax=857 但 displayPx=950 → 強制覆寫為 round(950/1.05)=905(對齊 OYY2605163)",
			reqNonTax: 857, displayPx: 950, taxMode: 1, taxRate: 5,
			want: 905,
		},
		{
			name:      "displayPx=0(贈品/樣品)→ 回 reqNonTax(零價穿透)",
			reqNonTax: 0, displayPx: 0, taxMode: 2, taxRate: 5,
			want: 0,
		},
		{
			name:      "漏送 reqNonTax=0,應稅 → 回 displayPx",
			reqNonTax: 0, displayPx: 752, taxMode: 2, taxRate: 5,
			want: 752,
		},
		{
			name:      "漏送 reqNonTax=0,含稅 rate=5,displayPx=210 → round(210/1.05)=200",
			reqNonTax: 0, displayPx: 210, taxMode: 1, taxRate: 5,
			want: 200,
		},
		{
			name:      "漏送 reqNonTax=0,含稅 rate=5,displayPx=950 → 905",
			reqNonTax: 0, displayPx: 950, taxMode: 1, taxRate: 5,
			want: 905,
		},
		{
			name:      "含稅 rate=0 退化 → 回 displayPx",
			reqNonTax: 0, displayPx: 100, taxMode: 1, taxRate: 0,
			want: 100,
		},
		{
			name:      "TaxMode=0(預設未填)→ 視為應稅,回 displayPx",
			reqNonTax: 0, displayPx: 657, taxMode: 0, taxRate: 5,
			want: 657,
		},
		{
			name:      "負 displayPx → 回 reqNonTax(零價穿透分支)",
			reqNonTax: 0, displayPx: -100, taxMode: 2, taxRate: 5,
			want: 0,
		},
		{
			name:      "非零 reqNonTax + 負 displayPx → 回 reqNonTax(零價穿透維持原值,corner case)",
			reqNonTax: 500, displayPx: -1, taxMode: 2, taxRate: 5,
			want: 500,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveNonTaxPrice(tc.reqNonTax, tc.displayPx, tc.taxMode, tc.taxRate)
			if got != tc.want {
				t.Errorf("ResolveNonTaxPrice(reqNonTax=%v, displayPx=%v, taxMode=%v, taxRate=%v) = %v, want %v",
					tc.reqNonTax, tc.displayPx, tc.taxMode, tc.taxRate, got, tc.want)
			}
		})
	}
}
