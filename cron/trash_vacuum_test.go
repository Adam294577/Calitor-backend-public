package cron

import (
	"project/services/storage"
	"testing"
	"time"
)

func TestPlanTrashVacuum(t *testing.T) {
	now := time.Date(2026, 8, 25, 3, 30, 0, 0, time.UTC)
	retention := 90 * 24 * time.Hour
	cutoff := now.Add(-retention) // 2026-05-27 03:30 UTC

	obj := func(key string, modAt time.Time) storage.ObjectInfo {
		return storage.ObjectInfo{Key: key, LastModified: modAt}
	}

	cases := []struct {
		name         string
		allTrash     []storage.ObjectInfo
		wantToDelete []string
		wantWarning  bool
	}{
		{
			name:     "空 trash 不刪",
			allTrash: nil,
		},
		{
			name: "全部過期 → 全真刪 + 觸發 warning(>50%)",
			allTrash: []storage.ObjectInfo{
				obj("products-trash/2026/05/01/a.jpg", cutoff.Add(-30*24*time.Hour)),
				obj("products-trash/2026/05/01/b.jpg", cutoff.Add(-30*24*time.Hour)),
			},
			wantToDelete: []string{
				"products-trash/2026/05/01/a.jpg",
				"products-trash/2026/05/01/b.jpg",
			},
			wantWarning: true,
		},
		{
			name: "混合:1 過期 + 1 未過期 → 只真刪過期那個,不觸發 warning(剛好 50% 不算 >50%)",
			allTrash: []storage.ObjectInfo{
				obj("products-trash/2026/05/01/old.jpg", cutoff.Add(-1*time.Hour)),
				obj("products-trash/2026/07/01/new.jpg", cutoff.Add(40*24*time.Hour)),
			},
			wantToDelete: []string{"products-trash/2026/05/01/old.jpg"},
			wantWarning:  false,
		},
		{
			name: "邊界:剛好 retention(cutoff 時刻)→ 不刪(嚴格 Before 才刪)",
			allTrash: []storage.ObjectInfo{
				obj("products-trash/2026/05/27/boundary.jpg", cutoff),
			},
			wantToDelete: nil,
			wantWarning:  false,
		},
		{
			name: "邊界:剛好過 cutoff 1 奈秒 → 刪",
			allTrash: []storage.ObjectInfo{
				obj("products-trash/2026/05/27/just-old.jpg", cutoff.Add(-time.Nanosecond)),
			},
			wantToDelete: []string{"products-trash/2026/05/27/just-old.jpg"},
			wantWarning:  true, // 1/1 = 100% > 50% → 必定觸發 warning(這是 vacuum 正常情況)
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := planTrashVacuum(tc.allTrash, now, retention)
			if !sameStringSet(got.ToDelete, tc.wantToDelete) {
				t.Fatalf("ToDelete mismatch: got %v, want %v", got.ToDelete, tc.wantToDelete)
			}
			if (got.Warning != "") != tc.wantWarning {
				t.Fatalf("Warning presence mismatch: got %q, want present=%v", got.Warning, tc.wantWarning)
			}
		})
	}
}
