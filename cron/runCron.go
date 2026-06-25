package cron

import (
	"project/services/log"
	"sync"

	"github.com/robfig/cron/v3"
	"github.com/spf13/viper"
)

var cronjob *cron.Cron
var cronMutex sync.Mutex

func Run() {
	cronMutex.Lock()
	defer cronMutex.Unlock()

	// 如果已經有跑的，先停掉
	if cronjob != nil {
		ctx := cronjob.Stop()
		<-ctx.Done() // 等待所有 goroutine 結束
		log.Info("Old cron stopped")
	}

	c := cron.New(cron.WithSeconds())
	//			  ┌─────────────── 秒     (0 - 59)
	//            | ┌───────────── 分鐘   (0 - 59)
	//            | │ ┌─────────── 小時   (0 - 23)
	//            | │ │ ┌───────── 日     (1 - 31)
	//            | │ │ │ ┌─────── 月     (1 - 12)
	//            | │ │ │ │ ┌───── 星期幾 (0 - 6，0 是週日，6 是週六，7 也是週日)
	//			  │ │ │ │ │ │
	// c.AddFunc("5 * * * * *", func())
	ctx := map[string]error{}
	// 每日凌晨 3:00 清理孤兒圖片(已加 Pluck err 檢查 + 空 result + 50% 比例上限 三層 safety guard,見 cleanup_orphan_images.go)
	// cleanup 現為「軟刪除」:把孤兒移到 products-trash/,90 天後由 TrashVacuum 真刪
	if _, err := c.AddFunc("0 0 3 * * *", CleanupOrphanImages); err != nil {
		ctx["cleanup_orphan_images"] = err
	}
	// 每日凌晨 3:30 真刪 trash 中超過 90 天的物件(在 cleanup 之後 30 分,錯開資源)
	if _, err := c.AddFunc("0 30 3 * * *", TrashVacuum); err != nil {
		ctx["trash_vacuum"] = err
	}

	if viper.GetString("ENV") == "prod" {

	}

	if len(ctx) != 0 {
		log.Debug("cron run error: %v", ctx)
	}
	if len(c.Entries()) > 0 {
		c.Start()
		log.Info("The Cron Jobs Running")
	} else {
		log.Info("No Cron Entries")
	}

	cronjob = c
}
