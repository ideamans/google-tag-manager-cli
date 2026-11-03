package gtm

import "testing"

func TestRetryable(t *testing.T) {
	cases := []struct {
		method string
		code   int
		want   bool
	}{
		{"GET", 500, true},
		{"PUT", 503, true},
		{"DELETE", 502, true},
		{"POST", 500, false}, // 作成・公開は二重実行を避ける
		{"POST", 429, true},  // 未処理なので安全
		{"GET", 404, false},
		{"POST", 400, false},
	}
	for _, c := range cases {
		if got := retryable(c.method, c.code); got != c.want {
			t.Errorf("%s %d: got %v", c.method, c.code, got)
		}
	}
}
