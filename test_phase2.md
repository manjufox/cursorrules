# Phase 2 テスト手順書 - 画像処理・アーカイブ展開機能

## 🎯 テスト対象機能

### 新規追加API
- `/api/v1/image/*path` - 画像配信（リサイズ対応）
- `/api/v1/archive/*path` - アーカイブ展開
- `/api/v1/thumbnail/*path` - サムネイル生成

### サポート機能
- 画像リサイズ（Lanczosアルゴリズム）
- ZIP/CBZ/RAR/CBR展開
- ディレクトリサムネイル

## 📋 テスト手順

### 1. サーバー起動確認

```bash
go run main.go
```

**期待結果:**
```
Configuration loaded - Source: S:\comic\saimin
Starting manga server on 0.0.0.0:8080
Manga source path: S:\comic\saimin
```

### 2. 基本機能テスト

#### 2.1 ヘルスチェック
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

#### 2.2 バージョン確認
```bash
curl http://localhost:8080/
```

**期待結果:**
```json
{
  "message": "Manga Server Phase 2",
  "version": "2.0.0", 
  "phase": "2 - Image Processing & Archive Support"
}
```

### 3. 画像配信テスト

#### 3.1 ディレクトリ内画像の直接配信
```bash
# 最初のディレクトリの最初の画像ファイルを取得
curl -I "http://localhost:8080/api/v1/image/(同人CG集) [サークルENZIN] 催眠王様ゲーム 後編/000.jpg"
```

**期待結果:**
- Status: 200 OK
- Content-Type: image/jpeg
- Cache-Control: public, max-age=3600

#### 3.2 画像リサイズテスト
```bash
# 幅800pxにリサイズ
curl -I "http://localhost:8080/api/v1/image/(同人CG集) [サークルENZIN] 催眠王様ゲーム 後編/000.jpg?width=800"

# 高さ600pxにリサイズ
curl -I "http://localhost:8080/api/v1/image/(同人CG集) [サークルENZIN] 催眠王様ゲーム 後編/000.jpg?height=600"

# 幅800px、高さ600px、品質75%でリサイズ
curl -I "http://localhost:8080/api/v1/image/(同人CG集) [サークルENZIN] 催眠王様ゲーム 後編/000.jpg?width=800&height=600&quality=75"
```

**期待結果:**
- すべて Status: 200 OK
- Content-Type: image/jpeg

### 4. アーカイブ展開テスト

#### 4.1 CBZファイル展開
```bash
# CBZファイルの内容を確認
curl "http://localhost:8080/api/v1/archive/(同人誌) [ぷにぷにのほっぺ (かわよい)] 催眠性教育 [DL版].cbz"
```

**期待結果:**
```json
{
  "files": [...],
  "count": 35,
  "archive_path": "(同人誌) [ぷにぷにのほっぺ (かわよい)] 催眠性教育 [DL版].cbz",
  "archive_type": ".cbz"
}
```

#### 4.2 その他アーカイブファイル
```bash
# ZIPファイル展開テスト
curl "http://localhost:8080/api/v1/archive/巻貝一ヶ催眠インストラクター紫吹.zip"
```

### 5. サムネイル生成テスト

#### 5.1 ディレクトリサムネイル
```bash
# ディレクトリの最初の画像をサムネイル化
curl -I "http://localhost:8080/api/v1/thumbnail/(同人CG集) [サークルENZIN] 催眠王様ゲーム 後編"
```

**期待結果:**
- Status: 200 OK
- Content-Type: image/jpeg
- 200x200px サムネイル

#### 5.2 アーカイブサムネイル
```bash
# CBZファイルの最初の画像をサムネイル化
curl -I "http://localhost:8080/api/v1/thumbnail/(同人誌) [ぷにぷにのほっぺ (かわよい)] 催眠性教育 [DL版].cbz"
```

#### 5.3 カスタムサイズサムネイル
```bash
# 300x300pxサムネイル
curl -I "http://localhost:8080/api/v1/thumbnail/(同人CG集) [サークルENZIN] 催眠王様ゲーム 後編?size=300"
```

### 6. エラーハンドリングテスト

#### 6.1 存在しないファイル
```bash
curl "http://localhost:8080/api/v1/image/nonexistent.jpg"
```

**期待結果:**
```json
{
  "error": "Image not found"
}
```

#### 6.2 画像以外のファイル
```bash
curl "http://localhost:8080/api/v1/image/some_text_file.txt"
```

**期待結果:**
```json
{
  "error": "Not an image file"
}
```

#### 6.3 サポートされていないアーカイブ
```bash
curl "http://localhost:8080/api/v1/archive/some_file.7z"
```

**期待結果:**
```json
{
  "error": "Unsupported archive format"
}
```

## 🎛️ パフォーマンステスト

### 応答時間測定
```bash
# 画像配信速度
time curl -s "http://localhost:8080/api/v1/image/(同人CG集) [サークルENZIN] 催眠王様ゲーム 後編/000.jpg" > /dev/null

# リサイズ速度
time curl -s "http://localhost:8080/api/v1/image/(同人CG集) [サークルENZIN] 催眠王様ゲーム 後編/000.jpg?width=800" > /dev/null

# アーカイブ展開速度
time curl -s "http://localhost:8080/api/v1/archive/(同人誌) [ぷにぷにのほっぺ (かわよい)] 催眠性教育 [DL版].cbz" > /dev/null

# サムネイル生成速度
time curl -s "http://localhost:8080/api/v1/thumbnail/(同人CG集) [サークルENZIN] 催眠王様ゲーム 後編" > /dev/null
```

**パフォーマンス目標:**
- 画像配信: < 1秒
- リサイズ: < 2秒
- アーカイブ展開: < 3秒
- サムネイル生成: < 2秒

## ✅ チェックリスト

### 基本機能
- [ ] サーバー起動
- [ ] ヘルスチェック
- [ ] バージョン情報

### 画像配信
- [ ] 直接画像配信
- [ ] 幅指定リサイズ
- [ ] 高さ指定リサイズ
- [ ] 幅・高さ指定リサイズ
- [ ] 品質指定

### アーカイブ展開
- [ ] CBZ展開
- [ ] ZIP展開
- [ ] CBR展開（もしファイルがあれば）
- [ ] RAR展開（もしファイルがあれば）

### サムネイル生成
- [ ] ディレクトリサムネイル
- [ ] アーカイブサムネイル
- [ ] カスタムサイズ
- [ ] デフォルトサイズ

### エラーハンドリング
- [ ] 存在しないファイル
- [ ] 画像以外のファイル
- [ ] サポートされていないアーカイブ
- [ ] 無効なパラメータ

### パフォーマンス
- [ ] 応答時間が目標内
- [ ] メモリ使用量が適切
- [ ] キャッシュが機能

## 🚨 已知の制限事項

1. **RAR暗号化**: パスワード付きRARファイルは未対応
2. **GIFアニメーション**: リサイズ時に静止画になる
3. **大容量ファイル**: 非常に大きな画像は処理に時間がかかる可能性
4. **メモリ使用量**: 複数の大きな画像を同時処理すると大量のメモリを使用

## 📝 テスト結果記録

テスト実行後、以下を記録してください：

```
テスト実行日時: ____
テスト実行者: ____

成功したテスト: __ / __
失敗したテスト: __ / __
平均応答時間: __ 秒
メモリ使用量: __ MB

コメント:
_________________________ 