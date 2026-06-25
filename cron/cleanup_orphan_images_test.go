package cron

import (
	"fmt"
	"project/services/storage"
	"testing"
	"time"
)

func TestPlanCleanup(t *testing.T) {
	now := time.Date(2026, 5, 27, 3, 0, 0, 0, time.UTC)
	grace := 24 * time.Hour
	cutoff := now.Add(-grace) // 2026-05-26 03:00 UTC

	obj := func(key string, modAt time.Time) storage.ObjectInfo {
		return storage.ObjectInfo{Key: key, LastModified: modAt}
	}

	cases := []struct {
		name           string
		allObjects     []storage.ObjectInfo
		usedURLs       []string
		maxRatio       float64
		wantToDelete   []string
		wantSkipped    int
		wantAbortLike  string // substring; "" 代表不該 abort
	}{
		{
			name: "所有物件都被 DB 引用 → 不刪也不跳",
			allObjects: []storage.ObjectInfo{
				obj("products/2026/05/20-a.jpg", cutoff.Add(-48*time.Hour)),
				obj("products/2026/05/26-b.jpg", cutoff.Add(time.Hour)),
			},
			usedURLs: []string{
				"products/2026/05/20-a.jpg",
				"products/2026/05/26-b.jpg",
			},
			maxRatio: 0.5,
		},
		{
			name: "所有物件都是孤兒但都在 grace 內 → 全跳,不刪",
			allObjects: []storage.ObjectInfo{
				obj("products/2026/05/26-x.jpg", cutoff.Add(time.Hour)),
				obj("products/2026/05/26-y.jpg", cutoff.Add(2*time.Hour)),
			},
			usedURLs:    []string{"products/2026/05/01-other.jpg"},
			maxRatio:    0.5,
			wantSkipped: 2,
		},
		{
			name: "混合:1 used + 1 孤兒新 + 1 孤兒舊 → 只刪舊孤兒",
			allObjects: []storage.ObjectInfo{
				obj("products/2026/05/26-used.jpg", cutoff.Add(time.Hour)),
				obj("products/2026/05/26-new-orphan.jpg", cutoff.Add(2*time.Hour)),
				obj("products/2026/05/20-old-orphan.jpg", cutoff.Add(-48*time.Hour)),
			},
			usedURLs:     []string{"products/2026/05/26-used.jpg"},
			maxRatio:     0.5,
			wantToDelete: []string{"products/2026/05/20-old-orphan.jpg"},
			wantSkipped:  1,
		},
		{
			name: "災難場景:DB 查回少數紀錄,大量物件變孤兒(>50%)→ abort,完全不刪",
			allObjects: []storage.ObjectInfo{
				obj("products/2026/05/20-a.jpg", cutoff.Add(-48*time.Hour)),
				obj("products/2026/05/20-b.jpg", cutoff.Add(-48*time.Hour)),
				obj("products/2026/05/20-c.jpg", cutoff.Add(-48*time.Hour)),
				obj("products/2026/05/20-d.jpg", cutoff.Add(-48*time.Hour)),
			},
			usedURLs:      []string{"products/2026/05/20-a.jpg"}, // 只剩 1 used,3 個孤兒舊 = 75%
			maxRatio:      0.5,
			wantAbortLike: "預計刪除 3/4",
		},
		{
			name: "邊界:刪除比例剛好 50% → 不 abort(嚴格大於才擋)",
			allObjects: []storage.ObjectInfo{
				obj("products/2026/05/20-a.jpg", cutoff.Add(-48*time.Hour)),
				obj("products/2026/05/20-b.jpg", cutoff.Add(-48*time.Hour)),
			},
			usedURLs:     []string{"products/2026/05/20-a.jpg"},
			maxRatio:     0.5,
			wantToDelete: []string{"products/2026/05/20-b.jpg"},
		},
		{
			name: "邊界:剛好超過 50% → abort",
			allObjects: []storage.ObjectInfo{
				obj("products/2026/05/20-a.jpg", cutoff.Add(-48*time.Hour)),
				obj("products/2026/05/20-b.jpg", cutoff.Add(-48*time.Hour)),
				obj("products/2026/05/20-c.jpg", cutoff.Add(-48*time.Hour)),
			},
			usedURLs:      []string{"products/2026/05/20-a.jpg"}, // 刪 2/3 = 66.7%
			maxRatio:      0.5,
			wantAbortLike: "預計刪除 2/3",
		},
		{
			name: "本次事件複現:usedURLs 為空(模擬 Pluck err 後未 abort 的舊行為),確認 ratio guard 仍能擋住",
			allObjects: []storage.ObjectInfo{
				obj("products/2026/05/21-old1.jpg", cutoff.Add(-100*time.Hour)),
				obj("products/2026/05/22-old2.jpg", cutoff.Add(-80*time.Hour)),
				obj("products/2026/05/25-old3.jpg", cutoff.Add(-50*time.Hour)),
				obj("products/2026/05/26-new1.jpg", cutoff.Add(2*time.Hour)),
			},
			usedURLs:      nil,
			maxRatio:      0.5,
			wantAbortLike: "預計刪除 3/4",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := planCleanup(tc.allObjects, tc.usedURLs, now, grace, tc.maxRatio)

			if tc.wantAbortLike != "" {
				if got.AbortReason == "" {
					t.Fatalf("expected abort containing %q, got no abort", tc.wantAbortLike)
				}
				if !contains(got.AbortReason, tc.wantAbortLike) {
					t.Fatalf("abort reason %q does not contain %q", got.AbortReason, tc.wantAbortLike)
				}
				if len(got.ToDelete) != 0 {
					t.Fatalf("abort should produce no ToDelete, got %v", got.ToDelete)
				}
				return
			}

			if got.AbortReason != "" {
				t.Fatalf("unexpected abort: %s", got.AbortReason)
			}
			if !sameStringSet(got.ToDelete, tc.wantToDelete) {
				t.Fatalf("ToDelete mismatch: got %v, want %v", got.ToDelete, tc.wantToDelete)
			}
			if got.SkippedNew != tc.wantSkipped {
				t.Fatalf("SkippedNew mismatch: got %d, want %d", got.SkippedNew, tc.wantSkipped)
			}
		})
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}

// TestRunCleanupMoves 覆蓋 2026-05-28 事件對應的 per-orphan 重驗證邏輯。
// 重點:即便 planCleanup 算出某 key 是孤兒(因 Pluck partial 漏列),
// 只要 verifier(單筆 SELECT EXISTS)回 true,就要救回來不 Move。
func TestRunCleanupMoves(t *testing.T) {
	// 構造 mover 同時記錄被呼叫的 (src, dst) pairs,方便驗證
	type movedPair struct{ src, dst string }
	newRecordingMover := func(failKeys map[string]bool) (orphanMover, *[]movedPair) {
		var calls []movedPair
		mover := func(src, dst string) error {
			calls = append(calls, movedPair{src, dst})
			if failKeys[src] {
				return fmt.Errorf("simulated Move failure for %s", src)
			}
			return nil
		}
		return mover, &calls
	}

	// 構造 verifier:given referenced set 與 err set
	newVerifier := func(referenced map[string]bool, errKeys map[string]bool) orphanVerifier {
		return func(key string) (bool, error) {
			if errKeys[key] {
				return false, fmt.Errorf("simulated verify error for %s", key)
			}
			return referenced[key], nil
		}
	}

	cases := []struct {
		name        string
		candidates  []string
		referenced  map[string]bool // 在 DB 中仍被引用(true → 救援)
		errKeys     map[string]bool // verifier 對該 key 回 err(保守跳過)
		failMoves   map[string]bool // mover 對該 key 回 err(Move 失敗)
		wantMoved   int
		wantAlarm   int
		wantVErr    int
		wantFailed  int
		wantMovedKeys []string // 真的被 Move 的 src key
	}{
		{
			name:       "全是真孤兒 → 全 Move",
			candidates: []string{"products/2026/05/20-a.jpg", "products/2026/05/20-b.jpg"},
			wantMoved:  2,
			wantMovedKeys: []string{"products/2026/05/20-a.jpg", "products/2026/05/20-b.jpg"},
		},
		{
			name:       "全是 referenced(全 Pluck 漏列救援)→ 0 Move",
			candidates: []string{"products/2026/05/20-a.jpg", "products/2026/05/20-b.jpg"},
			referenced: map[string]bool{
				"products/2026/05/20-a.jpg": true,
				"products/2026/05/20-b.jpg": true,
			},
			wantAlarm: 2,
		},
		{
			name: "**2026-05-28 事件複現**:11 候選中 10 個 Pluck 漏列(referenced)+ 1 真孤兒 → 只 Move 1 個",
			candidates: []string{
				"products/2026/05/26-1779774141533600301.jpg", // GB8048-01
				"products/2026/05/26-1779774155026313873.jpg", // 真孤兒(從未 referenced)
				"products/2026/05/26-1779781031566367266.jpg", // N9517W-01
				"products/2026/05/26-1779781058824204948.jpg", // N9517W-07
				"products/2026/05/26-1779782280410908562.jpg", // N9352W-07
				"products/2026/05/26-1779786395618217193.jpg", // GB7983-04
				"products/2026/05/26-1779786417590673603.jpg", // GB7983-11
				"products/2026/05/26-1779786449976489537.jpg", // GB7983-15
				"products/2026/05/26-1779786473434901725.jpg", // GB7983-17
				"products/2026/05/26-1779786532855100163.jpg", // GB8094-00
				"products/2026/05/26-1779786553214787497.jpg", // GB8094-01
			},
			referenced: map[string]bool{
				// 10 個 survivor 在 DB 仍被引用(audit-2 確認)
				"products/2026/05/26-1779774141533600301.jpg": true,
				"products/2026/05/26-1779781031566367266.jpg": true,
				"products/2026/05/26-1779781058824204948.jpg": true,
				"products/2026/05/26-1779782280410908562.jpg": true,
				"products/2026/05/26-1779786395618217193.jpg": true,
				"products/2026/05/26-1779786417590673603.jpg": true,
				"products/2026/05/26-1779786449976489537.jpg": true,
				"products/2026/05/26-1779786473434901725.jpg": true,
				"products/2026/05/26-1779786532855100163.jpg": true,
				"products/2026/05/26-1779786553214787497.jpg": true,
				// key #2 (1779774155026313873) 不在 referenced map → verifier 回 false → 真孤兒
			},
			wantMoved: 1,
			wantAlarm: 10,
			wantMovedKeys: []string{
				"products/2026/05/26-1779774155026313873.jpg",
			},
		},
		{
			name:       "verifier err → 保守跳過(寧可不搬)",
			candidates: []string{"products/2026/05/20-a.jpg", "products/2026/05/20-b.jpg"},
			errKeys: map[string]bool{
				"products/2026/05/20-a.jpg": true,
			},
			wantMoved: 1, // b 仍是真孤兒,Move 成功
			wantVErr:  1,
			wantMovedKeys: []string{"products/2026/05/20-b.jpg"},
		},
		{
			name:       "Move 失敗:真孤兒驗證通過但 mover 回 err → Failed 計數",
			candidates: []string{"products/2026/05/20-a.jpg", "products/2026/05/20-b.jpg"},
			failMoves: map[string]bool{
				"products/2026/05/20-a.jpg": true,
			},
			wantMoved:  1,
			wantFailed: 1,
			// mover 對兩個都被呼叫(a 失敗、b 成功)
			wantMovedKeys: []string{"products/2026/05/20-a.jpg", "products/2026/05/20-b.jpg"},
		},
		{
			name:       "空候選 → 0 之 0",
			candidates: nil,
			// 全 0,正常 noop
		},
	}

	const todayPrefix = "2026/05/29"
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verifier := newVerifier(tc.referenced, tc.errKeys)
			mover, callsPtr := newRecordingMover(tc.failMoves)

			got := runCleanupMoves(tc.candidates, verifier, mover, todayPrefix)

			if got.Moved != tc.wantMoved {
				t.Errorf("Moved: got %d, want %d", got.Moved, tc.wantMoved)
			}
			if got.FalseAlarm != tc.wantAlarm {
				t.Errorf("FalseAlarm: got %d, want %d", got.FalseAlarm, tc.wantAlarm)
			}
			if got.VerifyErr != tc.wantVErr {
				t.Errorf("VerifyErr: got %d, want %d", got.VerifyErr, tc.wantVErr)
			}
			if got.Failed != tc.wantFailed {
				t.Errorf("Failed: got %d, want %d", got.Failed, tc.wantFailed)
			}

			// 驗證 mover 真的只被「驗證通過的真孤兒」呼叫
			if tc.wantMovedKeys != nil {
				gotSrcs := make([]string, 0, len(*callsPtr))
				for _, p := range *callsPtr {
					gotSrcs = append(gotSrcs, p.src)
					// 同時驗證 dst 走 trash 路徑
					wantDst := "products-trash/" + todayPrefix + "/" + p.src
					if p.dst != wantDst {
						t.Errorf("dst: got %q, want %q", p.dst, wantDst)
					}
				}
				if !sameStringSet(gotSrcs, tc.wantMovedKeys) {
					t.Errorf("mover called with: got %v, want %v", gotSrcs, tc.wantMovedKeys)
				}
			} else if len(*callsPtr) != 0 {
				t.Errorf("mover not expected to be called, got %d calls", len(*callsPtr))
			}
		})
	}
}
