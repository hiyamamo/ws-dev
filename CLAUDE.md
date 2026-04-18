# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## リポジトリ概要

このリポジトリは **ws-dev CLI** の開発リポジトリ。Go 製の単一バイナリで、同一リポジトリを複数クローンして並列開発するワークフローを提供する。

- `cmd/ws-dev/` — エントリポイント（cobra ルート）
- `internal/` — 各機能モジュール（下記）
- `examples/` — 参考用の旧設定（Rails 向け Procfile/justfile、移行前の Node.js MCP 実装）

## ビルドとテスト

```bash
mise exec -- go build -o ws-dev ./cmd/ws-dev
mise exec -- go test ./...
mise exec -- go test ./internal/mcp -run TestSearchLog     # 単体テスト指定
```

Go は `mise` 経由で管理（`mise use -g go@latest` でインストール済み）。

## パッケージ構成と責務

| パッケージ | 責務 |
|------------|------|
| `internal/cmd` | cobra サブコマンド定義（init/clone/link/unlink/server/logs/run/mcp） |
| `internal/config` | `ws-dev.yml` のパース、`RepoName()` は URL から basename 抽出 |
| `internal/workspace` | `ws-dev.yml` 検出（cwd → 親へ遡上）、`RepoDir(label)` など |
| `internal/links` | `links/` → `repos/<repo>-<label>/` へ相対シムリンク作成。既存ディレクトリは置換 |
| `internal/tasks` | `exec_wrapper` を前置したコマンドを stdio 継承で実行 |
| `internal/procman` | Procfile 相当の並列プロセス管理。`{{.Label}}` 等を text/template で展開、各プロセスを setpgid で独立 pgid に配置、SIGTERM/SIGKILL でクリーンアップ |
| `internal/mcp` | stdio JSON-RPC の MCP サーバー実装（`list_logs` / `tail_log` / `truncate_log` / `search_log`） |

## 重要な設計ポイント

### サーバーは単一ラベル前提

`ws-dev server <label>` は `.ws-dev/server.pid` を読み、前回の ws-dev プロセスがまだ生きていれば `SIGTERM` で停止してから新プロセスを起動する。並列ラベル起動は想定していない（ポート衝突を避けるため）。

- `.ws-dev/server.pid` — 自身の PID
- `.ws-dev/current-label` — 直近ラベル（`ws-dev logs` が省略時に参照）

### ログディレクトリ解決順

コマンドラインフラグ `--log-dir` → 環境変数 `WS_DEV_LOG_DIR` → `ws-dev.yml` の `log_dir` → デフォルト `log`。

この順序は `server` / `logs` / `mcp` すべてで一貫している。`mcp` は `cwd` 基準で解決する（cwd は各クローンリポの中で実行される想定）。

### ディレクトリ命名

`repos/<repo-name>-<label>/`。`<repo-name>` は `repo:` URL の basename（`.git` 除去込み）から推定。ユーザーが渡す `<label>` は短い名前（例: `fix-login`）で、CLI 内部でプレフィックスを付加。

### プロセスの template 展開

`processes.<name>.cmd` は Go の `text/template` で評価される：

- `{{.Label}}` — ラベル
- `{{.PortBase}}` — `--port-base` / `$WS_DEV_PORT_BASE` / デフォルト 3000
- `{{.Workspace}}` — ワークスペース絶対パス

展開後にシェル的ホワイトスペース分割（`tasks.fields`）で argv 化する。クォート対応は最小限（`"..."` / `'...'` のみ、エスケープ非対応）。

## 手動検証のしかた

```bash
# サンプルワークスペースで一通り動かす
mkdir -p /tmp/ws-play && cd /tmp/ws-play
/path/to/ws-dev init sample && cd sample
# ws-dev.yml を編集（repo を実リポに、processes を埋める）
../ws-dev clone branch-a
../ws-dev link branch-a
../ws-dev server branch-a       # 別ターミナルで
../ws-dev logs                  # 直近ラベル
../ws-dev logs branch-a web -f  # follow
../ws-dev run branch-a console  # tasks.console
```

MCP 単体テスト：
```bash
cd repos/<repo>-branch-a
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n' | ws-dev mcp
```
