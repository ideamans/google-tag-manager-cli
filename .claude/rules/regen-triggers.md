---
paths:
  - "cmd/**/*.go"
  - "internal/gtm/audit.go"
  - "internal/gtm/entities.go"
  - "internal/llmdocs/0*.md"
  - "internal/llmdocs/1*.md"
---

# 埋め込みリファレンスの原本に触れました

コマンド・フラグ・ヘルプ文字列を変えたなら、終わる前に `/regen-ai` を実行して
`internal/llmdocs/90-commands.md` を一致させること。CI が再生成して差分が出たら
落ちます。

audit のコード（`internal/gtm/audit.go` の `Code*`）を増減したら、
`00-guide.md` の表と `cmd/audit.go` の `Long` も直すこと。
`TestGuideListsAuditCodes` が表の漏れを検出します。

生成物は直接編集しないこと。
