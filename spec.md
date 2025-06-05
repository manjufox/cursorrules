# Go製軽量Web漫画サイト仕様書 v2.0

## 📋 プロジェクト概要

### 目的

外出先でのVPN接続環境において、NASに保存された漫画ファイルを軽量かつ高速に閲覧できるWebサイトを構築する。

### 要件

- **軽量性**: 貧弱なネットワーク環境でも快適に動作
- **高速性**: ページ読み込み1秒以内、切り替え0.5秒以内
- **柔軟性**: あらゆるディレクトリ構造・ファイル形式に対応
- **シンプル性**: コア機能優先、段階的機能追加

## 🏗️ アーキテクチャ設計

### システム構成

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   NAS Storage   │───▶│   Go Backend    │───▶│   Web Client    │
│ (漫画ファイル)   │    │ (Docker環境)    │    │ (軽量フロント)   │
│ zip/rar/画像等  │    │ リアルタイム処理 │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 技術スタック

- **コンテナ**: Docker + Volume Mount
- **Webフレームワーク**: Gin (github.com/gin-gonic/gin)
- **言語**: Go 1.21+
- **画像処理**: `github.com/disintegration/imaging`
- **アーカイブ**: `archive/zip`, `github.com/nwaples/rardecode`
- **設定管理**: YAML設定ファイル
- **フロントエンド**: 軽量Vanilla JS + CSS

## 🏗️ Gin実装アーキテクチャ

### API設計
```go
// メインルーティング
r := gin.Default()

// 静的ファイル配信
r.Static("/assets", "./assets")

// API エンドポイント
api := r.Group("/api/v1")
{
    api.GET("/directories", listDirectories)      // ディレクトリ一覧
    api.GET("/files/:path", listFiles)           // ファイル一覧
    api.GET("/image/:path", serveImage)          // 画像配信
    api.GET("/archive/:path", extractArchive)    // アーカイブ展開
    api.GET("/thumbnail/:path", serveThumbnail)  // サムネイル
}

// ページルーティング
r.GET("/", indexPage)                           // トップページ
r.GET("/viewer/*path", viewerPage)              // ビューアページ
```

### ミドルウェア構成
```go
r.Use(gin.Logger())                    // アクセスログ
r.Use(gin.Recovery())                  // パニック復旧
r.Use(corsMiddleware())                // CORS対応
r.Use(cacheMiddleware())               // キャッシュ制御
r.Use(compressionMiddleware())         // gzip圧縮
```

## 📁 対応ファイル形式

### サポート形式

- **画像ファイル**: jpg, png, gif, webp
- **アーカイブファイル**: zip, rar, cbr, cbz
- **ディレクトリ構造**: 任意（自動認識）

### ディレクトリ例（柔軟対応）

```
/manga-library/
├── 作品1.zip                    # アーカイブファイル
├── 作品2/
│   ├── 001.jpg                  # 直接画像
│   ├── 002.png
│   └── ...
├── 作品3/
│   ├── vol01.cbz               # 巻ごとアーカイブ
│   ├── vol02.rar
│   └── ...
├── 作品4/
│   ├── 第1巻/
│   │   ├── 001.jpg
│   │   └── 002.jpg
│   └── 第2巻/
└── 混在ディレクトリ/            # 画像とアーカイブ混在
    ├── chapter1.zip
    ├── 001.jpg
    └── 002.png
```

## 🔧 主要機能

### 1. ファイル処理エンジン（最優先）

- **自動認識**: ディレクトリ構造の自動解析
- **アーカイブ展開**: zip/rar/cbr/cbz の動的展開
- **メタデータ抽出**: ファイル名・パスから情報解析
- **サムネイル生成**: ディレクトリ・ファイル一覧用

### 2. 画像最適化（シンプル設計）

- **基本リサイズ**: 画面サイズに応じた動的リサイズ
- **形式**: 元画像をそのまま配信（変換なし）
- **圧縮**: 必要に応じて品質調整
- **キャッシュ**: メモリキャッシュによる高速化

### 3. ユーザーインターフェース

- **ディレクトリナビ**: サムネイル表示でディレクトリ移動
- **ページビューア**: 画像表示・ページ送り
- **レスポンシブ**: デバイス対応（最優先）

### 4. キーボードナビゲーション（カスタマイズ可能）

```yaml
navigation:
  page_forward: ["ArrowRight", "Space", "j", "n", "PageDown"]
  page_backward: ["ArrowLeft", "Backspace", "k", "p", "PageUp"]
  first_page: ["Home", "Ctrl+Home", "g"]
  last_page: ["End", "Ctrl+End", "G"]
  next_chapter: ["Ctrl+ArrowRight", "Ctrl+n"]
  prev_chapter: ["Ctrl+ArrowLeft", "Ctrl+p"]
  file_list: ["Escape", "q"]
  fullscreen: ["f", "F11"]
  zoom_in: ["+", "Ctrl+="]
  zoom_out: ["-", "Ctrl+-"]
  fit_width: ["w"]
  fit_height: ["h"]
  actual_size: ["z"]
  single_page: ["1"]
  double_page: ["2"]
  vertical_scroll: ["v"]
  bookmark_add: ["b"]
  bookmark_list: ["Ctrl+b"]
  search: ["Ctrl+f", "/"]
  settings: ["s", "Ctrl+,"]
  page_jump: ["g+数字", "Ctrl+g"]
  auto_play: ["a"]
  speed_up: ["+"]
  speed_down: ["-"]
```

## 📊 パフォーマンス目標（強化）

### 高速化要件

- **ページ読み込み**: 1秒以内
- **ページ切り替え**: 0.5秒以内
- **サムネイル表示**: 2秒以内
- **アーカイブ展開**: 初回アクセス時のみ

### システム要件

- **サーバー負荷優先**: クライアント軽量化
- **メモリ使用量**: 制限なし（サーバー側）
- **CPU使用率**: 高負荷許容
- **キャッシュ戦略**: アグレッシブキャッシュ

## 🐳 Docker環境

### 構成

```dockerfile
FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o manga-server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/manga-server .
EXPOSE 8080
CMD ["./manga-server"]
```

### docker-compose.yml

```yaml
version: '3.8'
services:
  manga-server:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - /path/to/nas/manga:/manga:ro
    environment:
      - MANGA_PATH=/manga
      - PORT=8080
```

## 🛠️ 実装フェーズ（優先度調整）

### Phase 1: コア機能（最優先）

- [X] ファイル読み込み・解析（Priority 1）
- [ ] 基本的な画像処理（Priority 2）
- [ ] 簡易HTML生成（Priority 3）
- [ ] 基本的なHTTPサーバー（Priority 4）
- [ ] ディレクトリサムネイル表示
- [ ] アーカイブファイル対応

### Phase 2: UX強化

- [ ] キーボードナビゲーション（最優先）
- [ ] レスポンシブ対応（最優先）
- [ ] ブックマーク機能（Priority 2）
- [ ] 検索機能（Priority 3）

### Phase 3: 機能拡張

- [ ] 表示モード切替（1ページ・見開き・縦スクロール）
- [ ] ズーム・フィット機能
- [ ] 自動再生機能
- [ ] 設定画面

### Phase 4: 最適化

- [ ] パフォーマンス最適化
- [ ] キャッシュ強化
- [ ] ログ機能

## 📝 設定ファイル（簡素化）

```yaml
# config.yaml
server:
  port: 8080
  host: "0.0.0.0"
  
manga:
  source_path: "/manga"
  
image:
  max_width: 1920
  max_height: 1080
  quality: 85
  
cache:
  memory_cache: true
  max_cache_size: "1GB"
  
keyboard:
  customizable: true
  config_file: "keyboard.yaml"
```

## 🚀 デプロイメント

### Docker実行

```bash
# ビルド
docker build -t manga-server .

# 実行
docker run -d \
  -p 8080:8080 \
  -v /path/to/manga:/manga:ro \
  manga-server
```

### Docker Compose

```bash
docker-compose up -d
```

## 🔒 セキュリティ（簡素化）

### 基本方針

- **認証**: なし（後回し）
- **アクセス制御**: Docker内部ネットワーク
- **HTTPS**: 必要に応じて後で追加

## 📈 技術的制約・要件

### 負荷分散戦略

- **クライアント**: 最小限の処理のみ
- **サーバー**: 積極的な処理・キャッシュ
- **メモリ**: 制限なし、積極活用
- **CPU**: 高負荷許容

### 拡張性

- **モジュラー設計**: 機能の段階的追加
- **プラグイン対応**: 将来的な機能拡張
- **設定駆動**: YAMLによる柔軟な設定

---

## 🎯 次のステップ

1. **Docker環境セットアップ**
2. **ファイル読み込み・解析エンジン実装**
3. **アーカイブファイル対応**
4. **基本的なWebサーバー構築**
5. **サムネイル生成機能**

この仕様で実装を開始しますか？それとも追加で確認したい点はありますか？
