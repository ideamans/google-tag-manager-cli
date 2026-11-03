# gtmc — Google Tag Manager CLI

[![test](https://github.com/ideamans/google-tag-manager-cli/actions/workflows/test.yml/badge.svg)](https://github.com/ideamans/google-tag-manager-cli/actions/workflows/test.yml)
[![release](https://img.shields.io/github/v/release/ideamans/google-tag-manager-cli)](https://github.com/ideamans/google-tag-manager-cli/releases)
[![license](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

[English](README.md) | 日本語

Google Tag Manager (GTM) を操作する CLI（Go 製、コマンド名 `gtmc`）。コンテナの把握・棚卸し・エクスポートと、
ワークスペースでのタグ／トリガー／変数などの編集（一括編集・ギャラリーからのテンプレート導入を含む）、
バージョンの作成・公開・管理、ステージング環境への反映を扱う。API へのリクエストはすべてローカルの証跡ログに残る。
人が使うことと、AI エージェントに使わせることの両方を想定している。

- 管理者向けを除く Tag Manager API v2 の全メソッドに対応（110 中 96。残りはユーザー権限・アカウント設定・
  コンテナの作成/削除などの管理操作）
- 取り消しにくい操作は `--yes` が必須。冪等でないリクエストは自動で再送しない（通信が不安定でも二重に作成・公開しない）
- `gtmc llm` で AI エージェント向けの完全なリファレンスを出力する。Claude Code のスキルも同梱
- クライアント基盤: Google 公式 Go クライアント [`google.golang.org/api/tagmanager/v2`](https://pkg.go.dev/google.golang.org/api/tagmanager/v2)
- 認証: OAuth2 Installed App（refresh token 方式）
- API 仕様の一次情報源: [Tag Manager API v2](https://developers.google.com/tag-platform/tag-manager/api/v2)

```sh
gtmc auth login
gtmc container use GTM-XXXX
gtmc audit --format table                      # 公開中のコンテナの中身と問題点
gtmc workspace create --name "UAの撤去" --use
gtmc tag pause 42
gtmc workspace quick-preview                   # コンパイルの確認
gtmc version create --name "UAの撤去"
gtmc version publish 15 --yes                  # サイトが変わるのはこの手順だけ
```

> **状態:** v0.x。ウェブコンテナは実際の API で確認済み。サーバーコンテナでのクライアント・変換の作成と、
> `gtmc export` の出力を GTM の画面でインポートすることは未確認（報告を歓迎します）。

## インストール

```sh
go install github.com/ideamans/google-tag-manager-cli/cmd/gtmc@latest
```

Go 1.26 以上が必要。Go が無ければ [Releases](https://github.com/ideamans/google-tag-manager-cli/releases) から
`google-tag-manager_<version>_<os>_<arch>.tar.gz` を取得し、中の `gtmc` を PATH に置く。

## セットアップ

### 1. OAuth クライアントの用意

Google Cloud Console で対象プロジェクトの **Tag Manager API** を有効化し、**Desktop app** 種別の
OAuth2 クライアントの JSON を次のパスに保存する（`{"installed": {...}}` 形式のまま可）。

```
~/.config/google-tag-manager/oauth2-client.json
```

CLI はクライアントを同梱していない（利用者自身のプロジェクトで API の割り当てと同意画面を持つ）。`--client-id` / `--client-secret` や環境変数
`GTM_CLIENT_ID` / `GTM_CLIENT_SECRET` でも指定できる。

### 2. ログイン

```sh
gtmc auth login              # 参照・編集・バージョン作成・公開・ワークスペース削除
gtmc auth login --readonly   # 参照だけ（預かった他社コンテナの調査など）
```

refresh token は `~/.config/google-tag-manager/credentials.json`（0600）に保存される。

### 3. 疎通確認と既定のコンテナ

```sh
gtmc auth status
gtmc auth list-accounts
gtmc container use GTM-XXXX   # 以後 --container を省略できる
```

## 対象の指定

GTM の階層は **アカウント → コンテナ → ワークスペース → タグ／トリガー／変数**。

| 値 | フラグ | 環境変数 | ファイル |
|---|---|---|---|
| アカウント | `--account` | `GTM_ACCOUNT_ID` | `credentials.json` |
| コンテナ | `--container` | `GTM_CONTAINER_ID` | `credentials.json`（`container use`） |
| ワークスペース | `--workspace` | `GTM_WORKSPACE_ID` | `credentials.json`（`workspace use`） |

- `--container` は **`GTM-XXXX`**（アカウントID不要）、`accounts/<a>/containers/<c>`、
  数字のID（`--account` と併用）のどれでもよい
- ワークスペースを指定しないときは、コンテナにワークスペースが **1つだけなら** それを使う。
  複数あるときは推測せずエラーにする
- 保存済みのワークスペースは保存済みのコンテナにだけ適用する
- 設定ディレクトリは `GTM_HOME` で上書きできる

## 読む・棚卸しする

```sh
gtmc container list --all --format table      # 見えるコンテナすべて
gtmc container info GTM-XXXX
gtmc container snippet                        # サイトに貼る設置コード
gtmc tag list --format table                  # trigger / variable / folder / template なども同じ
gtmc tag get 42
gtmc version list --format table
gtmc version get live                         # 公開中のバージョン丸ごと

gtmc export --from live -o live.json          # GTM の UI でインポートできる形式
gtmc audit --format table                     # 棚卸し（変更はしない）
gtmc audit --file live.json                   # エクスポートをオフラインで棚卸し
```

`--from` は `live`（公開中）/ `latest`（最新バージョン）/ `workspace`（編集中）/ バージョン番号。

### 扱えるエンティティ

| コマンド | 対象 | 備考 |
|---|---|---|
| `tag` / `trigger` / `variable` | タグ・トリガー・ユーザー定義変数 | `tag pause` / `unpause` あり |
| `folder` | フォルダ | `move`（中へ移す）・`entities`（中身の一覧） |
| `template` | カスタムテンプレート | `import`（コミュニティギャラリーから導入） |
| `client` / `transformation` | クライアント・変換 | サーバーコンテナのみ |
| `zone` | ゾーン | ウェブコンテナ・GTM 360 のみ |
| `gtag-config` | Google タグの設定 | ウェブコンテナ。API に revert が無い |
| `builtin-variable` | 組み込み変数 | `enable` / `disable` / `revert` |

いずれも `list` / `get` / `create --file` / `update` / `delete` / `revert` を持つ。

`audit` が挙げるもの: 一時停止中のタグ、発火トリガーの無いタグ、どこからも参照されない
トリガー・変数、存在しない変数への参照（コンパイルできない）、Universal Analytics のタグ、
同一内容のカスタム HTML、`document.write` を使うカスタム HTML、同名のタグ。カスタム HTML は本文の大きい順に並ぶ（表示速度への影響の見積もり用）。

## 変更する

**編集はワークスペースの中で行われ、バージョンを作って公開するまでサイトには出ない。**

```sh
gtmc workspace create --name "UAの撤去" --use

gtmc tag pause 42                              # 一時停止（unpause で戻る）
gtmc tag update 42 --name "GA4 - page_view" --folder 7
gtmc tag get 42 > t.json && vi t.json && gtmc tag update 42 --file t.json
gtmc trigger create --file trigger.json        # - で標準入力
gtmc variable delete 13 --yes
gtmc tag revert 42                             # ワークスペースでの変更を取り消す
gtmc folder move 7 --tags 42,43
gtmc builtin-variable enable clickText clickUrl
gtmc workspace bulk-update --file changes.json # 複数の変更を1回で（new_1 などの仮IDで相互参照できる）
gtmc template import --owner <owner> --repository <repo> --acknowledge-permissions

gtmc workspace status --format table           # 変更点の一覧
gtmc workspace quick-preview                   # バージョンを作らずにコンパイルを確認（エラーなら終了コード1）
gtmc version create --name "UAの撤去" --notes "UAは2024年に計測を停止"
gtmc environment update 3 --version 15         # ステージング環境へ反映（本番は変わらない）
gtmc version publish 15 --yes                  # 公開（サイトに反映される）
```

- `update --file` は**全置換**。`get` で取った JSON を直して渡す
- `create --file` は本文の ID・パス・fingerprint を無視するので、別コンテナのエクスポートの
  要素を写せる。ただし**参照（トリガーID・フォルダID・`{{変数名}}`・`cvt_` テンプレート）は
  付け替えない**。参照ごと写すときは `workspace bulk-update` で `new_N` の仮IDを振る
- `workspace sync` で競合が出たら `workspace resolve-conflict --kind tag --id 12 --take workspace|base`
- `version create` はワークスペースを**消費する**（GTM の仕様）。新しいワークスペースが作られるのは
  それが最後の1つだったときだけ。保存済みの既定だった場合は新しいものへ切り替え、作られなかったときは
  既定を外す
- ギャラリーのテンプレートを使うタグの type は `cvt_<ギャラリーID>`（`template list --format table` の TYPE 列）
- `workspace quick-preview` がエラーのとき、API は理由をほぼ返さないので、CLI が参照切れの `{{変数}}` を
  `diagnostics` に示す
- 取り消しにくい操作は `--yes` が必須: 各エンティティの `delete`、`workspace delete`、削除を含む
  `workspace bulk-update`、`version publish`、`version delete`、`environment delete`、
  `environment reauthorize`
- 戻すときは `version list` で前のバージョン番号を見て `version publish <番号> --yes`
- バージョンの整理: `version update`（名前・メモ）、`version delete --yes` / `undelete`、`version set-latest`。
  公開中のバージョンは削除できない

### ステージング環境

```sh
gtmc environment create --name staging --url https://stg.example.com
gtmc environment list        # previewQuery を gtm.js の URL に足すとその環境が読み込まれる
gtmc environment update 3 --version 15
```

### この CLI で扱わないこと

ユーザー権限の管理、アカウント設定、コンテナの作成・削除・統合、Google タグ ID の移動、
送信先（destinations）の連携は GTM の画面で行う（管理者向けで、余分な権限を要求しないため）。
コンテナファイルのインポート、プレビューモード（Tag Assistant）、承認ワークフロー、
サーバーコンテナの実行環境の用意は API が無い。

## 証跡ログ

API を呼ぶコマンドはすべて、**どの対象にどんな操作をしたか**をローカルのログに残す。
意図しないアカウントやコンテナを触っていないことを後から確かめるためのもの。
ログを開けないときは API を呼ばずに失敗する（証跡の無い操作を作らない）。

- 場所: `~/.local/state/google-tag-manager/logs/audit-YYYY-MM-DD.jsonl`（`$XDG_STATE_HOME`、
  `$GTM_LOG_DIR` で変更可）。1日1ファイルの JSON Lines
- ローテーション: 90 日より古いファイルは次の実行時に自動で消える
  （`$GTM_LOG_RETENTION_DAYS` で変更可）
- 中身: コマンドの開始（引数・対象）と終了（成否）、API リクエスト1件ごとの
  メソッド・パス・アカウント/コンテナ/ワークスペース・状態コード・本文。
  長い文字列（256字超）と長い配列（50件超）は省略する。トークンと `--client-secret` は残さない

```sh
gtmc log dir                                         # ログのディレクトリ（パスだけ）
open "$(gtmc log dir)"                               # Finder で開く
gtmc log path                                        # ディレクトリ・ファイル一覧・保持日数（JSON）
gtmc log show --since 2h --writes                    # 変更系だけ
gtmc log show --since 24h --failed                   # 失敗したリクエスト・コマンドだけ
gtmc log show --invocation <id>                      # 1回の実行の全行（開始行にコマンドライン）
gtmc log summary --since 2026-11-03 --allow-account 1234567890   # 他のアカウントに触れていれば終了コード1
gtmc log summary --since 8h --allow-container 98765432           # コンテナ単位で縛る（数値の ID）
```

`write` は GET 以外すべてに付く。`:quick_preview` と `folders/N:entities` は POST だが読むだけ。
ログはこのマシン・この OS ユーザーの分だけで、GTM の画面での変更は入らない。
エージェントに監査させるときは `gtmc-log-audit` スキルを使う。

## LLM（AI エージェント）向け

```sh
gtmc llm             # GTM のモデル・安全な変更手順・JSON の形・全コマンドのカタログ
gtmc llm --format json
```

Claude Code ではマーケットプレイス（`ideamans/claude-public-plugins`）から
プラグインとして入れられる。スキルは `/gtmc-usage`（使い方と確認手順）、
`/gtmc-log-audit`（証跡ログの監査）、`/gtmc-install`（導入）。

```
/plugin marketplace add ideamans/claude-public-plugins
/plugin install google-tag-manager-cli@ideamans-plugins
```

Copilot・Cursor・Gemini CLI などでは同じスキルを `gh skill` で入れる。

```sh
gh skill install ideamans/google-tag-manager-cli/plugins/google-tag-manager-cli/skills/gtmc-usage --agent copilot
gh skill install ideamans/google-tag-manager-cli/plugins/google-tag-manager-cli/skills/gtmc-log-audit --agent copilot
gh skill install ideamans/google-tag-manager-cli/plugins/google-tag-manager-cli/skills/gtmc-install --agent copilot
```

## 開発・CI・リリース

```sh
go build -o gtmc ./cmd/gtmc
go generate ./...   # 埋め込みのコマンドカタログを作り直す
go test ./...
```

- **test**（`.github/workflows/test.yml`）… `go mod tidy` 差分 → vet → build → カタログ再生成の差分 → `go test -race`
- **release**（`.github/workflows/release.yml`）… `v*` タグの push で GoReleaser。タグと `PluginVersion` の一致を検査する

リリースの手順（バージョンの上げ方・マーケットプレイスへの登録）は
[`plugins/google-tag-manager-cli/PUBLISH.md`](plugins/google-tag-manager-cli/PUBLISH.md)。

## ライセンス

MIT（[LICENSE](LICENSE)）。Google Tag Manager は Google LLC の商標で、この CLI は Google の公式製品ではない。
