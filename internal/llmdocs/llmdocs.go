// Package llmdocs は `gtmc llm` が出力するリファレンスを埋め込む。
//
// 00-guide.md は手書き。90-commands.md は cobra のコマンドツリーから
// `go generate ./...` で生成する。go:embed がビルド時に実体を要求するため
// コミットしており、CI が再生成して差分が出たら失敗する。
package llmdocs

import (
	"embed"

	kit "github.com/ideamans/go-llm-cli-kit/llmdocs"
)

//go:generate go run ../gen-llmdocs

//go:embed *.md
var files embed.FS

// Docs は埋め込みリファレンス。
func Docs() *kit.Docs { return kit.New(files, ".") }
