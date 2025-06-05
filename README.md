# 🚀 Go製軽量Web漫画サーバー

外出先でのVPN接続環境において、NASに保存された漫画ファイルを軽量かつ高速に閲覧できるWebサイト。

## ✨ 主要機能

### 📚 ファイル対応
- **画像ファイル**: JPG, PNG, GIF, WebP
- **アーカイブファイル**: ZIP, RAR, CBZ, CBR
- **ディレクトリ構造**: 任意の入れ子構造に対応

### ⚡ 高速化機能
- **メモリキャッシュ**: 最大500ファイルまでキャッシュ（設定可能）
- **プリフェッチ**: 次の100ページを先読み（設定可能）
- **インテリジェントキャッシュ**: LRU方式で自動管理
- **リアルタイム進行状況**: プリフェッチの進行状況を表示

### 🎮 ユーザーインターフェース
- **レスポンシブデザイン**: PC・タブレット・スマホ対応
- **キーボードナビゲーション**: 矢印キー、スペース、ESCなど
- **表示モード**: シングル・見開き・縦スクロール
- **フィットモード**: 画面・幅・高さフィット
- **フルスクリーン対応**: F11キーまたはボタン

### 🔧 管理機能
- **設定ファイル**: YAML形式で柔軟な設定
- **API監視**: キャッシュ状況・プリフェッチ進行状況
- **ヘルスチェック**: サーバー状態の確認
- **詳細ログ**: アクセスログ・エラーログ

## 🚀 クイックスタート

### 1. 実行
```bash
# 直接実行
./manga-server.exe

# または環境変数で設定
MANGA_PATH=/path/to/your/manga ./manga-server.exe
```

### 2. アクセス
ブラウザで `http://localhost:8080` にアクセス

## ⚙️ 設定

### config.yaml
```yaml
# サーバー設定
server:
  host: "0.0.0.0"
  port: "8080"

# 漫画ファイルパス
manga:
  source_path: "S:/comic"

# キャッシュ設定
cache:
  max_size: 500                    # 最大キャッシュファイル数
  ttl_minutes: 60                  # キャッシュ保持時間（分）
  cleanup_interval_minutes: 10     # クリーンアップ間隔（分）

# プリフェッチ設定
prefetch:
  count: 100                       # プリフェッチページ数
  enabled: true                    # プリフェッチ有効/無効

# パフォーマンス設定
performance:
  image_quality: 85                # JPEG品質（1-100）
  max_image_width: 1920           # 最大画像幅
  max_image_height: 1080          # 最大画像高さ

# ログ設定
logging:
  level: "info"                    # ログレベル
  enable_access_log: true          # アクセスログ有効/無効
```

### 環境変数
- `MANGA_PATH`: 漫画ファイルのパス
- `PORT`: サーバーポート番号

## 🎯 キーボードショートカット

| キー | 機能 |
|------|------|
| `→` `Space` `j` `n` `PageDown` | 次のページ |
| `←` `Backspace` `k` `p` `PageUp` | 前のページ |
| `Home` `g` | 最初のページ |
| `End` `G` | 最後のページ |
| `Esc` `q` | ディレクトリに戻る |
| `f` `F11` | フルスクリーン切替 |
| `1` | シングルページモード |
| `2` | 見開きモード |
| `v` | 縦スクロールモード |

## 📊 API エンドポイント

### 基本API
- `GET /api/v1/health` - ヘルスチェック
- `GET /api/v1/directories` - ディレクトリ一覧
- `GET /api/v1/files/{path}` - ファイル一覧

### 画像配信API
- `GET /api/v1/image/{path}` - 画像配信
- `GET /api/v1/archive/{path}` - アーカイブ展開
- `GET /api/v1/archive-image/{path}` - アーカイブ内画像配信
- `GET /api/v1/thumbnail/{path}` - サムネイル生成

### 高速化API
- `GET /api/v1/prefetch/{path}` - プリフェッチ開始
- `GET /api/v1/prefetch-status/{path}` - プリフェッチ状況
- `GET /api/v1/cache-status` - キャッシュ状況

## 🐳 Docker対応

### Dockerfile
```dockerfile
FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o manga-server main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/manga-server .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/config.yaml .
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
      - /path/to/manga:/manga:ro
    environment:
      - MANGA_PATH=/manga
```

## 📈 パフォーマンス

### 目標値
- **ページ読み込み**: 1秒以内
- **ページ切り替え**: 0.5秒以内（キャッシュヒット時）
- **サムネイル表示**: 2秒以内

### 実測値（キャッシュ有効時）
- **初回アクセス**: ~1.4秒
- **2回目以降**: ~50ms（キャッシュヒット）
- **プリフェッチ済み**: 即座に表示

## 🔍 トラブルシューティング

### よくある問題

1. **ファイルが表示されない**
   - `config.yaml`の`source_path`を確認
   - ファイルパスの権限を確認

2. **画像が読み込まれない**
   - サポートされているファイル形式か確認
   - アーカイブファイルが破損していないか確認

3. **プリフェッチが動作しない**
   - `config.yaml`の`prefetch.enabled`を確認
   - メモリ使用量を確認

### ログ確認
```bash
# サーバーログを確認
tail -f /var/log/manga-server.log

# キャッシュ状況を確認
curl http://localhost:8080/api/v1/cache-status
```

## 🛠️ 開発

### 必要な依存関係
```bash
go mod tidy
```

### ビルド
```bash
go build -o manga-server main.go
```

### テスト
```bash
go test ./...
```

## 📝 ライセンス

MIT License

## 🤝 コントリビューション

プルリクエストやイシューの報告を歓迎します！

---

**🎉 高速で軽量な漫画リーダーをお楽しみください！** 