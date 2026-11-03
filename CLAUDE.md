# CLAUDE.md — gtmc

Google Tag Manager API v2 の CLI。**公開リポジトリ（MIT）**。名前が3つあるので取り違えないこと:

| 何 | 名前 |
| --- | --- |
| リポジトリ・Go モジュール・プラグイン | `google-tag-manager-cli`（`github.com/ideamans/google-tag-manager-cli`） |
| コマンド（バイナリ） | `gtmc`（`cmd/gtmc`。`go install …/cmd/gtmc@latest`）。`gtm` は別の OSS（owntag/gtm-cli）が同じ形のサブコマンドで使っているので避けた |
| goreleaser のプロジェクト・設定とログのディレクトリ | `google-tag-manager`（`~/.config/google-tag-manager`、`~/.local/state/google-tag-manager/logs`） |
| 環境変数 | `GTM_*`（`GTM_HOME`・`GTM_LOG_DIR`・`GTM_CLIENT_ID` など） |
| スキル | `gtmc-usage`・`gtmc-log-audit`・`gtmc-install` |

エージェントへの配布は `gtmc llm`・公開マーケットプレイス（`ideamans/claude-public-plugins`、
`google-tag-manager-cli@ideamans-plugins`）・`gh skill` の3経路。

解析（`audit` / `export`）と実作業（ワークスペースの編集・バージョン作成・公開）の両方を持つ。

**公開リポジトリなので、実在のアカウント ID・コンテナ ID・GTM-XXXX をコード・テスト・
ドキュメントに書かないこと**（例は `1234567890` や `GTM-XXXX` を使う）。

## 構成

| 場所 | 役割 |
| --- | --- |
| `internal/config` | 設定ファイルと、アカウント/コンテナ/ワークスペースの解決（フラグ > 環境変数 > ファイル） |
| `internal/gtm/client.go` | API クライアント、GTM-XXXX の解決（`containers.lookup`）、429/5xx の再試行 |
| `internal/gtm/entities.go` | エンティティ9種の `Kind`（`makeKind` で生成。CLI はこれから同形のサブコマンドを作る）。新しい種類は `Kinds` に足すだけ |
| `internal/gtm/admin.go` | ワークスペース（quick-preview・競合解消・一括更新）・バージョン・環境・設置コード |
| `internal/gtm/export.go` | `Snapshot`（live/latest/workspace/版番号）と UI インポート形式 |
| `internal/gtm/audit.go` | 棚卸し。API を呼ばない純粋関数（テストしやすい） |
| `internal/auditlog` | 証跡ログ（JSONL・日次ファイル・保持期間で自動削除）と、全 API リクエストを記録する `Transport` |

## 守っている約束（崩さないこと）

- **取り消しにくい操作は `--yes` 必須**: エンティティの `delete`、`workspace delete`、
  削除を含む `workspace bulk-update`、`version publish`、`version delete`、
  `environment delete`、`environment reauthorize`。`TestDestructiveCommandsRequireYes` が検査する。
  新しく破壊的な操作を足したら、ここと SKILL.md の確認表と `00-guide.md` の Ground rules にも足す
- **5xx の自動再送は GET・PUT・DELETE だけ**。POST（作成・バージョン作成・公開・一括更新）は
  処理済みかどうか分からないので再送しない（二重作成・二重公開を防ぐ。`TestRetryable`）。429 は常に再送
- **ギャラリーのテンプレート導入は `--acknowledge-permissions` 必須**（API の要求。権限を付与する操作）
- **管理者向けの API は実装しない**: ユーザー権限・アカウント設定・コンテナの作成/削除/統合・
  タグ ID の移動・destinations。`manage.users` / `manage.accounts` スコープを要求しないため。
  `00-guide.md` の "What this CLI will not do" に書いてある
- **ワークスペースを推測しない**。1つしか無いときだけ自動で選ぶ。保存済みの
  ワークスペースは保存済みのコンテナにしか適用しない（`resolve_test.go`）
- **作成時は本文の ID・パス・fingerprint を捨てる**（他コンテナのエクスポートから
  写せるように）。参照（トリガーID等）の付け替えはしない — ガイドに明記してある
- 本文の JSON の未知フィールドはエラー（綴り違いが黙って捨てられるのを防ぐ）
- **API への全リクエストを証跡ログに残す**。`auditlog.Transport` は再試行の内側（1回ごとに記録）。
  ログを開けなければ `gtm.NewClient` はクライアントを作らない（`TestClientRequiresAuditLog`）。
  API を呼ぶ新しい経路を作るときも必ず `NewClient` を通すこと。トークンと秘密のフラグは書かない
  （`TestTransportRecordsRequests`・`TestRedactArgs`）。ローテーションの設定（日数・サイズ）は未確定で、
  いまは日次ファイル＋90日保持。ログを読むのは `log dir/path/show/summary` と
  `gtmc-log-audit` スキル。`write` は GET 以外すべて（`:quick_preview` 等の読むだけの POST も含む）

## 変更時の必須手順

**機能を追加した、フラグを増やした、既存の挙動を変えた — このいずれかをしたら、
3か所すべてを更新してから終わること。**

| 更新先 | 対象 | やり方 |
| --- | --- | --- |
| ① ドキュメント | `README.md`・`README.ja.md` | 使い方が変わったとき（英語と日本語の両方） |
| ② ヘルプ | cobra の `Short` / `Long` / フラグ説明 | コード内。**カタログはここから生成される** |
| ③ **LLMナレッジ** | `internal/llmdocs/00-guide.md` | モデル・安全手順・JSON の形・audit のコード（手書き） |
| | `internal/llmdocs/90-commands.md` | **生成物。手編集しない** → `go generate ./...` |
| | `plugins/google-tag-manager-cli/skills/*/SKILL.md` | 手順・前提・確認手順が変わったとき |
| | `context7.json` の `rules` | エージェントが間違えやすい罠を見つけたとき |

- audit のコードを増減した → `00-guide.md` の表と `cmd/audit.go` の `Long`
  （`TestGuideListsAuditCodes` が漏れを検出）
- GTM の API で分かった罠（例: 版の作成でワークスペースが消える）→ `00-guide.md`
  の該当節と失敗モード表、SKILL.md の失敗モード表
- API カバー率の点検は Discovery 文書（`https://tagmanager.googleapis.com/$discovery/rest?version=v2`）
  の全メソッドとコード中の呼び出しを突き合わせる。2026-11-03 時点で管理者向け以外は全て実装済み

## 実地テストで分かった API の挙動（2026-11-03、テスト用のウェブコンテナで確認）

- fingerprint の不一致は 409 ではなく `400 The provided entity fingerprint is not valid.`
- 存在しないトリガー ID は書き込み時に 400 で拒まれる。存在しない `{{変数}}` は通り、コンパイル時に
  理由なしの `compilerError` になる → `quick-preview` の `diagnostics` と audit の
  `BROKEN_VARIABLE_REFERENCE` で補っている
- ギャラリーのテンプレートの type は `cvt_<galleryTemplateId>`（例 `cvt_MBTSV`）。自作は `cvt_<containerId>_<templateId>`
- `version create` が新しいワークスペースを作るのは最後の1つを消費したときだけ
- `built_in_variables.revert` の URL は `{workspace}/built_in_variables:revert`（パスにワークスペースを渡す）
- 利用者が作った環境は `type` が空で返る
- ウェブコンテナで `gtag-config create` に最小の本文を送ると 500（形が未確認）。zones は無料版で create が 404
- `version publish`（latest・番号）と切り戻し、`version delete/undelete` は確認済み。削除中のバージョンは
  `versions.get` が 404 で、`version_headers.list?includeDeleted=true` にだけ出る。公開中の版の削除は CLI が拒む
- 未検証: サーバーコンテナ（clients/transformations の作成）、エクスポートの UI へのインポート

## リリース

`PluginVersion`（`cmd/root.go`）と `plugin.json` の `version` と git タグの3つを
揃える。手順は `plugins/google-tag-manager-cli/PUBLISH.md`。

## 確認

```bash
go generate ./...     # コマンドカタログを作り直す
git diff --exit-code  # 差分が出たらコミット漏れ
go test ./...         # SKILL.md 検証・バージョン整合・--yes の検査を含む
go run . llm | head
```

## 参照

- 標準: <https://github.com/ideamans/go-llm-cli-kit/blob/main/LLM.md>
- 生成物と原本の対応: `.claude/rules/ai-artifacts-policy.md`
- API: <https://developers.google.com/tag-platform/tag-manager/api/v2>
