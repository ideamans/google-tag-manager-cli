---
name: regen-ai
description: 埋め込みLLMリファレンスのコマンドカタログを再生成して検証する。コマンド・フラグ・ヘルプ文を変更した後に使う。
allowed-tools: Bash(go generate:*) Bash(go test:*) Bash(go build:*) Bash(git status:*) Bash(git diff:*) Read
---

# regen-ai

`internal/llmdocs/90-commands.md` をコマンドツリーに合わせ直します。

1. `git status --short` — 既存の差分を把握する。
2. `go generate ./...` — `90-commands.md` を書き直す。
3. `go build ./... && go test ./...`。
4. どのコマンド・フラグが増減したかを報告する。

このスキルは Claude Code ローカル用で、配布プラグインには含みません。
