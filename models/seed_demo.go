package models

import (
	"fmt"
	"math"

	"project/services/log"
)

// SeedDemoData 灌入作品集 demo 用的業務假資料（幣別、尺碼、類別、廠商、客戶、商品、
// 廠商價格、尺碼庫存，以及進貨/銷售/訂貨單據）。
//
// 僅供公開 demo 版本使用：讓面試官接上空 DB + RUN_MIGRATE=true 後，登入即可看到有資料、
// 自洽（庫存/單據數字一致）的完整系統。以 products 表 count 做冪等，已有資料則整體略過。
func SeedDemoData(db *DBManager) {
	var productCount int64
	db.GetRead().Model(&Product{}).Count(&productCount)
	if productCount > 0 {
		return // 已有商品資料，視為已 seed，略過
	}

	// 記錄者：預設管理員（SeedDefaultAdmin 已先建立）
	var admin Admin
	db.GetRead().Where("account = ?", "admin").First(&admin)
	recorderID := admin.ID
	if recorderID == 0 {
		recorderID = 1
	}

	round2 := func(v float64) float64 { return math.Round(v*100) / 100 }
	i64 := func(v int64) *int64 { return &v }

	// --- 幣別 ---
	for _, c := range []Currency{
		{Code: "TWD", Name: "新台幣", Symbol: "NT$", ExchangeRate: 1, IsActive: true},
		{Code: "USD", Name: "美元", Symbol: "$", ExchangeRate: 32, IsActive: true},
		{Code: "EUR", Name: "歐元", Symbol: "€", ExchangeRate: 35, IsActive: true},
	} {
		db.GetWrite().Where("code = ?", c.Code).FirstOrCreate(&c)
	}

	// --- 尺碼組與尺碼選項 ---
	shoe := SizeGroup{Code: "SHOE", Name: "鞋碼"}
	db.GetWrite().Where("code = ?", shoe.Code).FirstOrCreate(&shoe)
	var shoeOpts []SizeOption
	for i, s := range []string{"35", "36", "37", "38", "39", "40", "41", "42"} {
		o := SizeOption{SizeGroupID: shoe.ID, Code: s, Label: s, SortOrder: i + 1}
		db.GetWrite().Where("size_group_id = ? AND code = ?", shoe.ID, s).FirstOrCreate(&o)
		shoeOpts = append(shoeOpts, o)
	}
	cloth := SizeGroup{Code: "CLOTH", Name: "服飾尺碼"}
	db.GetWrite().Where("code = ?", cloth.Code).FirstOrCreate(&cloth)
	for i, s := range []string{"S", "M", "L", "XL"} {
		o := SizeOption{SizeGroupID: cloth.ID, Code: s, Label: s, SortOrder: i + 1}
		db.GetWrite().Where("size_group_id = ? AND code = ?", cloth.ID, s).FirstOrCreate(&o)
	}

	// --- 商品類別（5 層各數筆，供類別管理畫面有資料）---
	for _, c := range []ProductCategory1{{Code: "B01", Name: "自有品牌"}, {Code: "B02", Name: "代理品牌"}, {Code: "B03", Name: "聯名"}} {
		db.GetWrite().Where("code = ?", c.Code).FirstOrCreate(&c)
	}
	for _, c := range []ProductCategory2{{Code: "S01", Name: "休閒鞋"}, {Code: "S02", Name: "正式鞋"}, {Code: "S03", Name: "運動鞋"}, {Code: "S04", Name: "靴款"}} {
		db.GetWrite().Where("code = ?", c.Code).FirstOrCreate(&c)
	}
	for _, c := range []ProductCategory3{{Code: "G01", Name: "男款"}, {Code: "G02", Name: "女款"}, {Code: "G03", Name: "中性"}} {
		db.GetWrite().Where("code = ?", c.Code).FirstOrCreate(&c)
	}
	for _, c := range []ProductCategory4{{Code: "M01", Name: "真皮"}, {Code: "M02", Name: "合成皮"}, {Code: "M03", Name: "帆布"}} {
		db.GetWrite().Where("code = ?", c.Code).FirstOrCreate(&c)
	}
	for _, c := range []ProductCategory5{{Code: "Y01", Name: "2026SS"}, {Code: "Y02", Name: "2026FW"}} {
		db.GetWrite().Where("code = ?", c.Code).FirstOrCreate(&c)
	}

	// --- 廠商（10 家）---
	vendorNames := []string{"鴻昇皮件", "永大製鞋", "立信材料", "宏遠貿易", "金葉實業", "大同鞋業", "順發皮革", "華新貿易", "和泰製造", "三益企業"}
	var vendors []Vendor
	for i, name := range vendorNames {
		v := Vendor{
			Code: fmt.Sprintf("V%03d", i+1), Name: name + "有限公司", ShortName: name,
			IsVisible: true, TaxRate: 5, Discount: 100, ClosingDate: 26,
			ContactPerson: "業務", Phone1: fmt.Sprintf("02-2900%04d", i+1),
		}
		db.GetWrite().Where("code = ?", v.Code).FirstOrCreate(&v)
		vendors = append(vendors, v)
	}

	// --- 客戶（10 家，code 同時作為庫點）---
	customerNames := []string{"台北旗艦店", "信義門市", "板橋門市", "桃園門市", "台中門市", "新竹門市", "高雄門市", "台南門市", "新莊門市", "中壢門市"}
	var customers []RetailCustomer
	for i, name := range customerNames {
		c := RetailCustomer{
			Code: fmt.Sprintf("C%03d", i+1), Name: name, ShortName: name,
			IsVisible: true, TaxRate: 5, TaxMode: 2, Discount: 100, ClosingDate: 26,
			CreditLimit: 500000, ContactPerson: "店長", Phone1: fmt.Sprintf("02-2700%04d", i+1),
		}
		db.GetWrite().Where("code = ?", c.Code).FirstOrCreate(&c)
		customers = append(customers, c)
	}

	// --- 商品（25 筆，含廠商價格與尺碼庫存）---
	var products []Product
	for i := 1; i <= 25; i++ {
		base := float64(1000 + i*100) // 批價（未稅）
		p := Product{
			ModelCode:        fmt.Sprintf("ES%04d", 1000+i),
			NameSpec:         fmt.Sprintf("示範鞋款 %02d 真皮綁帶", i),
			Currency:         "TWD",
			MSRP:             round2(base * 2),
			Wholesale:        base,
			WholesaleTaxIncl: round2(base * 1.05),
			TradeMode:        1,
			IsVisible:        true,
			Season:           "2026SS",
			Size1GroupID:     i64(shoe.ID),
			MaterialOuter:    "牛皮",
		}
		db.GetWrite().Where("model_code = ?", p.ModelCode).FirstOrCreate(&p)
		products = append(products, p)

		// 廠商價格（主廠商）
		v := vendors[i%len(vendors)]
		pv := ProductVendor{
			ProductID: p.ID, VendorID: v.ID,
			CostStart: round2(base * 0.6), CostLast: round2(base * 0.6),
			CostDiscount: 100, IsPrimary: true,
		}
		db.GetWrite().Where("product_id = ? AND vendor_id = ?", p.ID, v.ID).FirstOrCreate(&pv)

		// 尺碼庫存（落在某一庫點）
		cust := customers[i%len(customers)]
		for si, opt := range shoeOpts {
			qty := (i*3+si*2)%20 + 5 // 5~24
			pss := ProductSizeStock{ProductID: p.ID, CustomerID: cust.ID, SizeOptionID: opt.ID, Qty: qty}
			db.GetWrite().
				Where("product_id = ? AND customer_id = ? AND size_option_id = ?", p.ID, cust.ID, opt.ID).
				FirstOrCreate(&pss)
		}
	}

	// 共用：建立一行明細的尺碼數量（涵蓋全部鞋碼，每碼 perSize）
	buildSizes := func(perSize int) (sizes []StockItemSize, total int) {
		for _, opt := range shoeOpts {
			sizes = append(sizes, StockItemSize{SizeOptionID: opt.ID, Qty: perSize})
			total += perSize
		}
		return
	}

	// --- 進貨單（8 張，數字自洽）---
	for s := 1; s <= 8; s++ {
		cust := customers[s%len(customers)]
		ven := vendors[s%len(vendors)]
		var items []StockItem
		for k := 0; k < 2; k++ {
			p := products[(s*2+k)%len(products)]
			perSize := 5 + k*3
			sizes, total := buildSizes(perSize)
			items = append(items, StockItem{
				ProductID:     p.ID,
				SizeGroupID:   i64(shoe.ID),
				ItemOrder:     k + 1,
				AdvicePrice:   p.MSRP,
				PurchasePrice: p.Wholesale,
				NonTaxPrice:   p.Wholesale,
				TotalQty:      total,
				TotalAmount:   round2(p.Wholesale * float64(total)),
				Sizes:         sizes,
			})
		}
		st := Stock{
			StockNo: fmt.Sprintf("PI20260601%03d", s), StockDate: "20260601",
			CustomerID: cust.ID, VendorID: ven.ID, RecorderID: recorderID,
			StockMode: 1, DealMode: 1, TaxMode: 2, TaxRate: 5, DiscountPercent: 100,
			InputMode: StockInputModeKeyboard, Items: items,
		}
		if err := db.GetWrite().Create(&st).Error; err != nil {
			log.Error("seed 進貨單失敗 %s: %s", st.StockNo, err.Error())
		}
	}

	// --- 零售銷售單（10 張）---
	for s := 1; s <= 10; s++ {
		cust := customers[s%len(customers)]
		p := products[(s*3)%len(products)]
		o1 := shoeOpts[s%len(shoeOpts)]
		o2 := shoeOpts[(s+1)%len(shoeOpts)]
		totalQty := 3
		amount := round2(p.MSRP * float64(totalQty))
		item := RetailSellItem{
			ItemOrder: 1, ProductID: p.ID, SizeGroupID: i64(shoe.ID),
			SellPrice: p.MSRP, TotalQty: totalQty, TotalAmount: amount,
			CashAmount: amount, SellMode: 1,
			Sizes: []RetailSellItemSize{{SizeOptionID: o1.ID, Qty: 2}, {SizeOptionID: o2.ID, Qty: 1}},
		}
		rs := RetailSell{
			SellNo: fmt.Sprintf("SE20260610%03d", s), SellDate: "20260610",
			CustomerID: cust.ID, SellStore: cust.Code, RecorderID: recorderID,
			TaxRate: 5, CashAmount: amount, InvoiceAmount: amount,
			Items: []RetailSellItem{item},
		}
		if err := db.GetWrite().Create(&rs).Error; err != nil {
			log.Error("seed 銷售單失敗 %s: %s", rs.SellNo, err.Error())
		}
	}

	// --- 客戶訂貨單（6 張，未交狀態）---
	for s := 1; s <= 6; s++ {
		cust := customers[s%len(customers)]
		p := products[(s*4)%len(products)]
		var sizes []OrderItemSize
		total := 0
		for _, opt := range shoeOpts {
			sizes = append(sizes, OrderItemSize{SizeOptionID: opt.ID, Qty: 4})
			total += 4
		}
		item := OrderItem{
			ProductID: p.ID, SizeGroupID: i64(shoe.ID), ItemOrder: 1,
			AdvicePrice: p.MSRP, OrderPrice: p.Wholesale, NonTaxPrice: p.Wholesale,
			TotalQty: total, TotalAmount: round2(p.Wholesale * float64(total)),
			CancelFlag: 1, ExpectedDate: "20260620", Sizes: sizes,
		}
		od := Order{
			OrderNo: fmt.Sprintf("OD20260605%03d", s), OrderDate: "20260605",
			CustomerID: cust.ID, RecorderID: recorderID, OrderStore: cust.Code,
			DealMode: 1, TaxMode: 2, TaxRate: 5, DeliveryStatus: 0,
			Items: []OrderItem{item},
		}
		if err := db.GetWrite().Create(&od).Error; err != nil {
			log.Error("seed 訂貨單失敗 %s: %s", od.OrderNo, err.Error())
		}
	}

	log.Info("已灌入 demo 業務資料：商品 25、廠商 10、客戶 10、進貨 8、銷售 10、訂貨 6")
}
