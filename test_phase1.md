# Phase 1 テスト手順書

## 🎯 Phase 1 の機能
- ✅ ファイル読み込み・解析（Priority 1）
- ✅ 基本的なGinサーバー
- ✅ サポートファイル形式の認識
- ✅ ディレクトリ・ファイル一覧API

## 🏗️ ビルド・実行手順

### 1. ローカル実行（開発環境）
```bash
# 依存関係インストール
go mod tidy

# サーバー起動
go run main.go
```

### 2. Docker実行
```bash
# イメージビルド
docker build -t manga-server .

# コンテナ実行
docker run -p 8080:8080 -v "S:/comic/saimin:/manga:ro" manga-server
```

### 3. Docker Compose実行
```bash
# 本番環境
docker-compose up --build

# 開発環境（ホットリロード）
docker-compose --profile dev up --build
```

## 🧪 健全性テスト

### API エンドポイントテスト

#### 1. ヘルスチェック
```bash
curl http://localhost:8080/api/v1/health
```

**期待結果:**
```json
{
  "status": "healthy",
  "source_path": "S:\\comic\\saimin",
  "phase": "1"
}
```

#### 2. ルートページ
```bash
curl http://localhost:8080/
```

**期待結果:**
```json
{
  "message": "Manga Server Phase 1",
  "version": "1.0.0",
  "phase": "1 - File Reading & Analysis"
}
```

#### 3. ディレクトリ一覧取得
```bash
curl http://localhost:8080/api/v1/directories
```

**期待結果:**
```json
{
  "directories": [
    {
      "name": "サブディレクトリ名",
      "path": "サブディレクトリ名",
      "is_dir": true,
      "size": 0,
      "extension": ""
    }
  ],
  "count": 1,
  "base_path": "S:\\comic\\saimin"
}
```

#### 4. ファイル一覧取得
```bash
# ルートディレクトリのファイル
curl http://localhost:8080/api/v1/files/

# サブディレクトリのファイル
curl http://localhost:8080/api/v1/files/サブディレクトリ名
```

**期待結果:**
```json
{
  "files": [
    {
      "name": "001.jpg",
      "path": "001.jpg",
      "is_dir": false,
      "size": 1024000,
      "extension": ".jpg"
    },
    {
      "name": "chapter1.zip",
      "path": "chapter1.zip", 
      "is_dir": false,
      "size": 5120000,
      "extension": ".zip"
    }
  ],
  "count": 2,
  "path": "/",
  "full_path": "S:\\comic\\saimin"
}
```

## 🔍 検証ポイント

### 1. ファイル形式認識
- ✅ 画像ファイル: `.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`
- ✅ アーカイブファイル: `.zip`, `.rar`, `.cbr`, `.cbz`
- ✅ ディレクトリの正しい認識

### 2. パス処理
- ✅ Windows パス（`S:\comic\saimin`）の正しい処理
- ✅ 相対パス・絶対パスの適切な結合
- ✅ パストラバーサル攻撃の基本対策

### 3. エラーハンドリング
- ✅ 存在しないパスへのアクセス
- ✅ 権限不足の場合の適切なエラー応答
- ✅ 不正なリクエストへの対応

### 4. パフォーマンス
- ✅ ディレクトリスキャンの応答速度
- ✅ メモリ使用量の確認
- ✅ 大量ファイルへの対応

## 🚨 既知の制限事項

1. **Phase 1では未実装の機能:**
   - 画像表示機能
   - アーカイブファイル展開
   - サムネイル生成
   - キャッシュ機能

2. **次Phase以降で対応予定:**
   - WebUIの実装
   - 画像リサイズ・最適化
   - キーボードナビゲーション

## 📊 成功基準

- [x] サーバーが正常に起動する
- [x] ヘルスチェックがPASSする
- [x] `S:\comic\saimin` のファイル・ディレクトリが正しく読み込める
- [x] サポートファイル形式が適切に認識される
- [x] APIレスポンスが仕様通りの形式で返される
- [x] Dockerコンテナが正常に動作する

## 🎯 Phase 2 への移行判断基準

全てのテストがPASSし、`S:\comic\saimin`の内容が正しく解析できればPhase 2（画像処理）に進行可能です。 