package common

import "testing"

func TestNormalizeModelCode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"小寫轉大寫", "gb8019-1", "GB8019-1"},
		{"混合大小寫", "Gb8019a", "GB8019A"},
		{"去頭尾空白", "  gb8019 ", "GB8019"},
		{"保留中間空白", " vivo sample ", "VIVO SAMPLE"},
		{"純數字不變", "8019", "8019"},
		{"含連字號", "gb8019-15", "GB8019-15"},
		{"全空白", "   ", ""},
		{"空字串", "", ""},
		{"已是大寫", "GB8030-01", "GB8030-01"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NormalizeModelCode(c.in); got != c.want {
				t.Errorf("NormalizeModelCode(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
