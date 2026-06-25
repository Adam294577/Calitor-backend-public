package middlewares

import (
	"bytes"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestLoginLimitKeyUser(t *testing.T) {
	cases := []struct {
		account string
		ip      string
		want    string
	}{
		{"alpha", "1.2.3.4", "login_limit:user:alpha:1.2.3.4"},
		{"  alpha  ", "1.2.3.4", "login_limit:user:alpha:1.2.3.4"}, // TrimSpace
		{"Mix-Case", "::1", "login_limit:user:Mix-Case:::1"},       // 大小寫保留(PG case-sensitive)
	}
	for _, c := range cases {
		if got := loginLimitKeyUser(c.account, c.ip); got != c.want {
			t.Errorf("loginLimitKeyUser(%q, %q) = %q; want %q", c.account, c.ip, got, c.want)
		}
	}
}

func TestLoginLimitKeyNonexistent(t *testing.T) {
	if got := loginLimitKeyNonexistent("1.2.3.4"); got != "login_limit:nonexistent:1.2.3.4" {
		t.Errorf("loginLimitKeyNonexistent IPv4 unexpected: %q", got)
	}
	if got := loginLimitKeyNonexistent("2407:4800::1"); got != "login_limit:nonexistent:2407:4800::1" {
		t.Errorf("loginLimitKeyNonexistent IPv6 unexpected: %q", got)
	}
}

// newCtxWithBody 模擬一個帶 body 的 Gin context
func newCtxWithBody(body string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/admin/login", strings.NewReader(body))
	return c
}

func TestExtractAccountFromBody_HappyPath(t *testing.T) {
	c := newCtxWithBody(`{"account":"alpha","password":"x"}`)
	if got := extractAccountFromBody(c); got != "alpha" {
		t.Errorf("happy path: got %q, want %q", got, "alpha")
	}
}

func TestExtractAccountFromBody_TrimsWhitespace(t *testing.T) {
	c := newCtxWithBody(`{"account":"  alpha  ","password":"x"}`)
	if got := extractAccountFromBody(c); got != "alpha" {
		t.Errorf("trim: got %q, want %q", got, "alpha")
	}
}

func TestExtractAccountFromBody_EmptyBody(t *testing.T) {
	c := newCtxWithBody(``)
	if got := extractAccountFromBody(c); got != "" {
		t.Errorf("empty body: got %q, want empty", got)
	}
}

func TestExtractAccountFromBody_MalformedJSON(t *testing.T) {
	c := newCtxWithBody(`{not json`)
	if got := extractAccountFromBody(c); got != "" {
		t.Errorf("malformed json: got %q, want empty", got)
	}
}

func TestExtractAccountFromBody_MissingAccountField(t *testing.T) {
	c := newCtxWithBody(`{"password":"x"}`)
	if got := extractAccountFromBody(c); got != "" {
		t.Errorf("missing account: got %q, want empty", got)
	}
}

// 確保讀完 body 後仍可被後續 handler 再次讀取(controller 端 ShouldBindJSON 依賴此行為)
func TestExtractAccountFromBody_BodyIsRestored(t *testing.T) {
	original := `{"account":"alpha","password":"secret"}`
	c := newCtxWithBody(original)
	_ = extractAccountFromBody(c)

	// 第二次讀 body,應拿到完整原始內容
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, c.Request.Body); err != nil {
		t.Fatalf("read body after extract: %v", err)
	}
	if buf.String() != original {
		t.Errorf("body not restored: got %q, want %q", buf.String(), original)
	}
}
