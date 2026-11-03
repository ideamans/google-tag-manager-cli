package cmd

import (
	"github.com/ideamans/go-llm-cli-kit/llmcmd"

	"github.com/ideamans/google-tag-manager-cli/internal/llmdocs"
)

// LLMConfig は `gtmc llm` サブコマンドの設定。
// 非推奨の --llm フラグをコマンドラインのどの位置でも受けるためにも使う。
func LLMConfig() llmcmd.Config {
	return llmcmd.Config{Docs: llmdocs.Docs()}
}
