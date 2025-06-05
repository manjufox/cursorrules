FROM golang:1.21-alpine AS builder

# 作業ディレクトリ設定
WORKDIR /app

# 依存関係ファイルをコピー
COPY go.mod go.sum ./

# 依存関係をダウンロード
RUN go mod download

# ソースコードをコピー
COPY . .

# バイナリをビルド
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o manga-server .

# 実行用のイメージ
FROM alpine:latest

# 必要なパッケージをインストール
RUN apk --no-cache add ca-certificates

# 作業ディレクトリ設定
WORKDIR /root/

# ビルドしたバイナリをコピー
COPY --from=builder /app/manga-server .

# ポート公開
EXPOSE 8080

# 実行
CMD ["./manga-server"] 