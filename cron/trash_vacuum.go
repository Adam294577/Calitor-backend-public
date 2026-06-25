package cron

import (
	"fmt"
	"project/services/log"
	"project/services/storage"
	"time"
)

const trashRetention = 90 * 24 * time.Hour

// trashVacuumPlan 是 planTrashVacuum 的決策輸出
type trashVacuumPlan struct {
	ToDelete []string
	Warning  string // 非 abort,只是警告(例:過期比例 > 50% 仍進行)
}

// planTrashVacuum 純函式:給定 trash 全物件 + 當下時間 + retention,
// 回傳要真刪的 key 清單。trash vacuum 不需 DB 對比,因為 trash 中物件本來就是「等真刪」狀態。
func planTrashVacuum(allTrash []storage.ObjectInfo, now time.Time, retention time.Duration) trashVacuumPlan {
	cutoff := now.Add(-retention)
	var toDelete []string
	for _, obj := range allTrash {
		if obj.LastModified.Before(cutoff) {
			toDelete = append(toDelete, obj.Key)
		}
	}

	var warning string
	if len(allTrash) > 0 && float64(len(toDelete))/float64(len(allTrash)) > 0.5 {
		warning = fmt.Sprintf("vacuum 預計刪除 %d/%d (>50%%) trash 物件;此為正常情況(累積 retention 期一次到期),但仍提示注意",
			len(toDelete), len(allTrash))
	}
	return trashVacuumPlan{ToDelete: toDelete, Warning: warning}
}

// TrashVacuum 真刪 products-trash/ 中超過 retention 期(90 天)的物件。
// 排程在 cleanup 之後 30 分(03:30 Taipei)觸發,避免兩 job 同時搶 MinIO ListObjects。
func TrashVacuum() {
	log.Info("[Cron] 開始 trash vacuum...")

	minioClient := storage.NewClient()
	if !minioClient.IsAvailable() {
		log.Warn("[Cron] MinIO 未啟用,跳過 trash vacuum")
		return
	}

	allTrash, err := minioClient.ListObjectsWithInfo("products-trash/")
	if err != nil {
		log.Error("[Cron] 列出 trash 物件失敗,中止 vacuum: %s", err.Error())
		return
	}
	if len(allTrash) == 0 {
		log.Info("[Cron] trash 為空,跳過 vacuum")
		return
	}

	plan := planTrashVacuum(allTrash, time.Now(), trashRetention)
	if plan.Warning != "" {
		log.Warn("[Cron] %s", plan.Warning)
	}

	deleted := 0
	failed := 0
	for _, key := range plan.ToDelete {
		if err := minioClient.Delete(key); err != nil {
			log.Warn("[Cron] trash vacuum 真刪失敗 %s: %s", key, err.Error())
			failed++
		} else {
			deleted++
		}
	}

	log.Info("[Cron] trash vacuum 完成:trash 共 %d 物件,真刪(超過 %d 天)%d 個,失敗 %d 個",
		len(allTrash), int(trashRetention/(24*time.Hour)), deleted, failed)
}
