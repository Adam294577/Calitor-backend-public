package cron

import (
	"fmt"
	"project/models"
	"project/services/log"
	"project/services/storage"
	"time"
)

const (
	cleanupGracePeriod    = 24 * time.Hour
	cleanupMaxDeleteRatio = 0.5
)

// cleanupPlan 是 planCleanup 的決策輸出
type cleanupPlan struct {
	ToDelete    []string
	SkippedNew  int
	AbortReason string
}

// orphanVerifier 對單一 key 重新確認是否仍被 DB 引用(true = 仍 referenced,應跳過 Move)
type orphanVerifier func(key string) (bool, error)

// orphanMover 把 srcKey 物件搬到 dstKey
type orphanMover func(srcKey, dstKey string) error

// moveResult 是 runCleanupMoves 的執行摘要
type moveResult struct {
	Moved      int // 真的搬到 trash 的數量
	FalseAlarm int // Pluck 漏列救援:planCleanup 算為孤兒,但重驗證發現仍被引用
	VerifyErr  int // 重驗證查詢失敗,保守跳過(寧可不搬)
	Failed     int // 重驗證確認是孤兒但 Move 失敗
}

// planCleanup 是 cleanup 決策的純函式版本(無副作用,純記憶體計算,方便測試)
// 給定 MinIO 物件清單 + DB 在用清單 + 當下時間 + grace + 比例上限,
// 回傳「應該刪哪些 key」「跳過幾個 grace 內物件」「是否該整批 abort」
func planCleanup(allObjects []storage.ObjectInfo, usedURLs []string, now time.Time,
	grace time.Duration, maxDeleteRatio float64) cleanupPlan {

	usedSet := make(map[string]bool, len(usedURLs))
	for _, u := range usedURLs {
		usedSet[u] = true
	}
	cutoff := now.Add(-grace)

	var toDelete []string
	skipped := 0
	for _, obj := range allObjects {
		if usedSet[obj.Key] {
			continue
		}
		if obj.LastModified.After(cutoff) {
			skipped++
			continue
		}
		toDelete = append(toDelete, obj.Key)
	}

	if len(toDelete) > 0 &&
		float64(len(toDelete))/float64(len(allObjects)) > maxDeleteRatio {
		return cleanupPlan{
			AbortReason: fmt.Sprintf("預計刪除 %d/%d (>%.0f%%) 疑似誤判,中止 cleanup 避免大規模誤殺",
				len(toDelete), len(allObjects), maxDeleteRatio*100),
		}
	}

	return cleanupPlan{ToDelete: toDelete, SkippedNew: skipped}
}

// runCleanupMoves 對 planCleanup 算出的孤兒候選逐一做 per-orphan 重驗證再 Move,
// 防止 Pluck partial 漏列(2026-05-28 事件主因)造成 referenced 檔被誤搬。
// 即便 bulk Pluck 漏掉了某個 image_url,單筆 SELECT EXISTS 仍會把它撈出來救援。
//
// 純函式(無 db / minio 直連),verifier / mover 由呼叫端注入,方便測試。
func runCleanupMoves(candidates []string, verifier orphanVerifier, mover orphanMover, todayPrefix string) moveResult {
	var r moveResult
	for _, key := range candidates {
		exists, err := verifier(key)
		if err != nil {
			// 驗證失敗保守跳過,寧可不搬(下次 cron 再試)
			r.VerifyErr++
			log.Warn("[Cron] re-verify %s 失敗,保守跳過: %s", key, err.Error())
			continue
		}
		if exists {
			// Pluck 漏列救援:重驗證發現此 key 仍被引用 → 跳過
			r.FalseAlarm++
			log.Warn("[Cron] Pluck 漏列救援:%s 經單筆查詢確認仍被引用,跳過 Move", key)
			continue
		}
		trashKey := fmt.Sprintf("products-trash/%s/%s", todayPrefix, key)
		if err := mover(key, trashKey); err != nil {
			r.Failed++
			log.Warn("[Cron] 移孤兒圖片到 trash 失敗 %s -> %s: %s", key, trashKey, err.Error())
		} else {
			r.Moved++
		}
	}
	return r
}

// CleanupOrphanImages 清理 MinIO 中無對應商品的孤兒圖片
func CleanupOrphanImages() {
	log.Info("[Cron] 開始清理孤兒圖片...")

	minioClient := storage.NewClient()
	if !minioClient.IsAvailable() {
		log.Warn("[Cron] MinIO 未啟用,跳過孤兒圖片清理")
		return
	}

	allObjects, err := minioClient.ListObjectsWithInfo("products/")
	if err != nil {
		log.Error("[Cron] 列出 MinIO 物件失敗: %s", err.Error())
		return
	}
	if len(allObjects) == 0 {
		log.Info("[Cron] MinIO 無任何商品圖片,跳過")
		return
	}

	db := models.PostgresNew()
	defer db.Close()
	var usedURLs []string
	if err := db.GetRead().Model(&models.Product{}).
		Where("image_url IS NOT NULL AND image_url != ''").
		Pluck("image_url", &usedURLs).Error; err != nil {
		log.Error("[Cron] 查詢 image_url 失敗,中止 cleanup 避免誤刪: %s", err.Error())
		return
	}
	if len(usedURLs) == 0 {
		log.Warn("[Cron] DB image_url 查詢回空 (MinIO 有 %d 物件),疑似異常,中止 cleanup 避免誤刪",
			len(allObjects))
		return
	}

	plan := planCleanup(allObjects, usedURLs, time.Now(), cleanupGracePeriod, cleanupMaxDeleteRatio)
	if plan.AbortReason != "" {
		log.Error("[Cron] %s", plan.AbortReason)
		return
	}

	// 軟刪除:把孤兒移到 products-trash/<刪除日期>/<原 key>,90 天後由 TrashVacuum 真刪。
	// 即使 cleanup 邏輯誤判(guard 失守),trash 仍給 90 天 recovery window。
	//
	// 額外:每個候選 key 在 Move 前都做 per-orphan 重驗證(單筆 SELECT EXISTS),
	// 防止 Pluck 在 cron 當下出現 partial 回傳(2026-05-28 事件主因)造成 referenced 檔被誤搬。
	todayPrefix := time.Now().Format("2006/01/02")
	verifier := func(key string) (bool, error) {
		var exists bool
		err := db.GetRead().Raw(
			"SELECT EXISTS(SELECT 1 FROM products WHERE image_url = ? AND deleted_at IS NULL)",
			key,
		).Scan(&exists).Error
		return exists, err
	}
	result := runCleanupMoves(plan.ToDelete, verifier, minioClient.Move, todayPrefix)

	log.Info("[Cron] 孤兒圖片清理完成:MinIO 共 %d 張,DB 使用 %d 張,移至 trash %d 張,跳過(未滿24h) %d 張,Pluck 漏列救援 %d 張,驗證失敗保守跳過 %d 張,Move 失敗 %d 張",
		len(allObjects), len(usedURLs), result.Moved, plan.SkippedNew, result.FalseAlarm, result.VerifyErr, result.Failed)
}
