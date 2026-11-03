package gtm

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"

	"google.golang.org/api/googleapi"
)

// apiError は Google API（REST）のエラーを読みやすい形に整える。
// googleapi.Error なら HTTP ステータス・メッセージ・詳細を展開し、
// それ以外はそのままラップする。
func apiError(prefix string, err error) error {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		msg := strings.TrimSpace(gerr.Message)
		if msg == "" {
			msg = bodySummary(gerr.Body)
		}
		line := fmt.Sprintf("%s: [%d] %s", prefix, gerr.Code, msg)

		// 詳細（field violations 等）があれば付記する。
		var details []string
		for _, item := range gerr.Errors {
			d := strings.TrimSpace(item.Message)
			if d == "" || d == msg {
				continue
			}
			if item.Reason != "" {
				d = fmt.Sprintf("[%s] %s", item.Reason, d)
			}
			details = append(details, d)
		}
		if len(details) > 0 {
			line += "\n  - " + strings.Join(details, "\n  - ")
		}
		switch {
		case gerr.Code == 403 && strings.Contains(strings.ToLower(msg), "scope"):
			line += "\n  hint: the saved token lacks this scope. If you logged in with --readonly, run `gtmc auth login` again without it"
		case gerr.Code == 403 && (strings.Contains(msg, "has not been used in project") || strings.Contains(msg, "it is disabled")):
			line += "\n  hint: enable the Tag Manager API in the Cloud project that owns the OAuth client (the URL above), wait a few minutes, and retry"
		case gerr.Code == 403:
			line += "\n  hint: the Google account has no permission on this GTM account/container, or the Tag Manager API is not enabled in the OAuth client's Cloud project"
		case gerr.Code == 409 || strings.Contains(strings.ToLower(msg), "fingerprint"):
			line += "\n  hint: the entity changed since it was read (fingerprint mismatch). Fetch it again and retry"
		}
		return &APIError{Code: gerr.Code, msg: line}
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

// APIError は整形済みの API エラー。HTTP ステータスを保持する。
type APIError struct {
	Code int
	msg  string
}

func (e *APIError) Error() string { return e.msg }

// StatusCode は API エラーの HTTP ステータスを返す（API エラーでなければ 0）。
func StatusCode(err error) int {
	var e *APIError
	if errors.As(err, &e) {
		return e.Code
	}
	return 0
}

var (
	htmlTitleRe = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
	htmlCodeRe  = regexp.MustCompile(`(?is)<code>(.*?)</code>`)
)

// bodySummary は JSON でない応答（Google の HTML エラーページなど）を1行に縮める。
func bodySummary(body string) string {
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "<") {
		if len(body) > 300 {
			return body[:300] + "…"
		}
		return body
	}
	parts := []string{}
	if m := htmlTitleRe.FindStringSubmatch(body); m != nil {
		parts = append(parts, strings.TrimSpace(m[1]))
	}
	if m := htmlCodeRe.FindStringSubmatch(body); m != nil {
		parts = append(parts, "URL "+html.UnescapeString(m[1]))
	}
	if len(parts) == 0 {
		return "non-JSON error response"
	}
	return strings.Join(parts, ": ")
}
