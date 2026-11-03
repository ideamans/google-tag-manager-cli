# 生成物 — 手編集しないこと

| 生成物 | 原本 | 再生成 |
| --- | --- | --- |
| `internal/llmdocs/90-commands.md` | `cmd/*.go` の cobra 定義 | `/regen-ai`（`go generate ./...`） |

手書き（編集してよい）:

- `internal/llmdocs/00-guide.md` — GTM のモデル、対象の解決、安全な変更手順、JSON の形、audit のコード
- `plugins/google-tag-manager-cli/skills/*/SKILL.md`

生成物を直接編集しても次の再生成で消え、それまでは CI が古い差分で落ちる。
カタログを良くしたいときはコマンドの `Short` / `Long` / フラグ説明を直すこと。
