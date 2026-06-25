package middlewares

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"project/services/log"
	"project/services/redis"
	response "project/services/responses"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	loginRateLimitMax = 5               // 最大失敗次數
	loginRateLimitTTL = 5 * time.Minute // 鎖定時間窗口
)

// 計數 key 結構說明
//   - login_limit:user:<account>:<ip>     由「帳號存在 + 密碼錯誤」遞增
//                                         綁定 (帳號, IP) 雙因子,避免同 IP 共用連坐
//                                         (痛點 A),也避免同帳號被遠端 DoS(痛點 B)
//   - login_limit:nonexistent:<ip>        僅由「帳號不存在」遞增
//                                         擋帳號列舉攻擊,且不影響存在帳號的登入流程
func loginLimitKeyUser(account, ip string) string {
	return fmt.Sprintf("login_limit:user:%s:%s", strings.TrimSpace(account), ip)
}

func loginLimitKeyNonexistent(ip string) string {
	return fmt.Sprintf("login_limit:nonexistent:%s", ip)
}

// extractAccountFromBody 從 request body 提取 account 欄位
// 讀完後用 io.NopCloser(bytes.NewBuffer(...)) 把 body 復原,讓 controller 端
// ShouldBindJSON 仍可正常 binding。Content-Type 非 JSON / body 為空 / 解析
// 失敗一律回空字串,呼叫方應視為「沒拿到帳號」處理。
func extractAccountFromBody(c *gin.Context) string {
	if c.Request.Body == nil {
		return ""
	}
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	if len(bodyBytes) == 0 {
		return ""
	}
	var body struct {
		Account string `json:"account"`
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.Account)
}

// LoginRateLimit 登入頻率限制(Redis-based)
//
// 計數單位:(帳號, IP) 組合 + 獨立 nonexistent IP
//   - 同辦公室不同帳號的失敗互不影響(解痛點 A)
//   - 同帳號從不同 IP 的失敗互不影響(解痛點 B,遠端 DoS)
//   - 大量不存在帳號列舉 → nonexistent IP 鎖,且不影響存在帳號
func LoginRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		rds := redis.Global()
		if !rds.IsAvailable() {
			c.Next()
			return
		}

		ip := c.ClientIP()
		account := extractAccountFromBody(c)
		resp := response.New(c)

		// 1. 若請求帶了 account,檢查 (account, ip) 是否已鎖
		if account != "" {
			key := loginLimitKeyUser(account, ip)
			if blocked, remaining := loginRateLimitIsBlocked(rds, key); blocked {
				log.Warn("LoginRateLimit: (帳號=%s, IP=%s) 已被鎖定,剩餘 %d 秒", account, ip, remaining)
				resp.Fail(http.StatusTooManyRequests,
					fmt.Sprintf("登入嘗試過多,請 %d 秒後再試", remaining)).Send()
				c.Abort()
				return
			}
		}

		// 2. 永遠檢查 nonexistent IP key,擋帳號列舉攻擊
		keyN := loginLimitKeyNonexistent(ip)
		if blocked, remaining := loginRateLimitIsBlocked(rds, keyN); blocked {
			log.Warn("LoginRateLimit: IP=%s 不存在帳號嘗試過多,剩餘 %d 秒", ip, remaining)
			resp.Fail(http.StatusTooManyRequests,
				fmt.Sprintf("登入嘗試過多,請 %d 秒後再試", remaining)).Send()
			c.Abort()
			return
		}

		c.Next()
	}
}

// loginRateLimitIsBlocked 檢查指定 key 是否已達上限,回傳 (是否擋, 剩餘秒數)
func loginRateLimitIsBlocked(rds *redis.Client, key string) (bool, int) {
	countStr, err := rds.Get(key)
	if err != nil {
		return false, 0
	}
	var count int
	fmt.Sscanf(countStr, "%d", &count)
	if count < loginRateLimitMax {
		return false, 0
	}
	ttl, _ := rds.TTL(key)
	remaining := int(ttl.Seconds())
	if remaining < 0 {
		remaining = 0
	}
	return true, remaining
}

// LoginRateLimitIncrUser 帳號存在 + 密碼錯誤時呼叫
// 對 (account, ip) 計數 +1,第一次失敗時設 TTL
func LoginRateLimitIncrUser(account, ip string) {
	loginRateLimitIncrAndExpire(loginLimitKeyUser(account, ip))
}

// LoginRateLimitIncrNonexistent 帳號不存在時呼叫
// 對 nonexistent:ip 計數 +1
func LoginRateLimitIncrNonexistent(ip string) {
	loginRateLimitIncrAndExpire(loginLimitKeyNonexistent(ip))
}

// LoginRateLimitResetUser 登入成功時呼叫
// 只重置 (account, ip) 計數;nonexistent IP 計數保持不變
// (避免某使用者一次成功登入順便幫攻擊者清掉列舉計數)
func LoginRateLimitResetUser(account, ip string) {
	rds := redis.Global()
	if !rds.IsAvailable() {
		return
	}
	rds.Delete(loginLimitKeyUser(account, ip))
}

// loginRateLimitIncrAndExpire 共用內部 helper:INCR + 第一次失敗時設 TTL
func loginRateLimitIncrAndExpire(key string) {
	rds := redis.Global()
	if !rds.IsAvailable() {
		return
	}
	count, err := rds.Increment(key)
	if err != nil {
		log.Error("LoginRateLimit INCR 失敗: %s", err.Error())
		return
	}
	if count == 1 {
		rds.Expire(key, loginRateLimitTTL)
	}
}
