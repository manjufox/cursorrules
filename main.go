package main

import (
	"archive/zip"
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
	"github.com/nwaples/rardecode"
	"gopkg.in/yaml.v2"
)

// Config 設定構造体
type Config struct {
	Server struct {
		Port string `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"server"`
	Manga struct {
		SourcePath string `yaml:"source_path"`
	} `yaml:"manga"`
	Cache struct {
		MaxSize               int `yaml:"max_size"`
		TTLMinutes           int `yaml:"ttl_minutes"`
		CleanupIntervalMinutes int `yaml:"cleanup_interval_minutes"`
	} `yaml:"cache"`
	Prefetch struct {
		Count   int  `yaml:"count"`
		Enabled bool `yaml:"enabled"`
	} `yaml:"prefetch"`
	Performance struct {
		ImageQuality   int `yaml:"image_quality"`
		MaxImageWidth  int `yaml:"max_image_width"`
		MaxImageHeight int `yaml:"max_image_height"`
	} `yaml:"performance"`
	Logging struct {
		Level           string `yaml:"level"`
		EnableAccessLog bool   `yaml:"enable_access_log"`
	} `yaml:"logging"`
}

// FileInfo ファイル情報構造体
type FileInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"is_dir"`
	Size      int64  `json:"size"`
	Extension string `json:"extension"`
}

// CacheEntry キャッシュエントリ
type CacheEntry struct {
	Data      []byte
	Timestamp time.Time
}

// ImageCache 画像キャッシュ
type ImageCache struct {
	cache map[string]*CacheEntry
	mutex sync.RWMutex
	maxSize int
	ttl time.Duration
}

// PrefetchStatus プリフェッチ状況
type PrefetchStatus struct {
	ArchivePath string `json:"archive_path"`
	TotalImages int    `json:"total_images"`
	Prefetched  int    `json:"prefetched"`
	InProgress  bool   `json:"in_progress"`
	StartTime   time.Time `json:"start_time"`
}

var config Config
var imageCache *ImageCache
var prefetchStatus map[string]*PrefetchStatus
var prefetchMutex sync.RWMutex

func main() {
	// 設定初期化
	initConfig()
	
	// キャッシュ初期化
	initCache()
	
	// 定期的なキャッシュクリーンアップを開始
	go func() {
		cleanupInterval := time.Duration(config.Cache.CleanupIntervalMinutes) * time.Minute
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				imageCache.Cleanup()
				log.Printf("Cache cleanup completed")
			}
		}
	}()
	
	// Ginルーター初期化
	r := gin.Default()
	
	// ミドルウェア設定
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	
	// ルーティング設定
	setupRoutes(r)
	
	// サーバー起動
	log.Printf("Starting manga server on %s:%s", config.Server.Host, config.Server.Port)
	log.Printf("Manga source path: %s", config.Manga.SourcePath)
	
	if err := r.Run(config.Server.Host + ":" + config.Server.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func initConfig() {
	// デフォルト設定
	setDefaultConfig()
	
	// 設定ファイルを読み込み
	if err := loadConfigFile("config.yaml"); err != nil {
		log.Printf("Warning: Could not load config file: %v", err)
		log.Printf("Using default configuration")
	}
	
	// 環境変数で設定を上書き
	if sourcePath := os.Getenv("MANGA_PATH"); sourcePath != "" {
		config.Manga.SourcePath = sourcePath
	}
	if port := os.Getenv("PORT"); port != "" {
		config.Server.Port = port
	}
	
	log.Printf("Configuration loaded - Source: %s, Host: %s, Port: %s", 
		config.Manga.SourcePath, config.Server.Host, config.Server.Port)
}

func setDefaultConfig() {
	config.Server.Host = "0.0.0.0"
	config.Server.Port = "8080"
	config.Manga.SourcePath = "S:/comic"
	config.Cache.MaxSize = 500
	config.Cache.TTLMinutes = 60
	config.Cache.CleanupIntervalMinutes = 10
	config.Prefetch.Count = 100
	config.Prefetch.Enabled = true
	config.Performance.ImageQuality = 85
	config.Performance.MaxImageWidth = 1920
	config.Performance.MaxImageHeight = 1080
	config.Logging.Level = "info"
	config.Logging.EnableAccessLog = true
}

func loadConfigFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	
	return yaml.Unmarshal(data, &config)
}

// キャッシュ初期化
func initCache() {
	imageCache = &ImageCache{
		cache:   make(map[string]*CacheEntry),
		maxSize: config.Cache.MaxSize,
		ttl:     time.Duration(config.Cache.TTLMinutes) * time.Minute,
	}
	prefetchStatus = make(map[string]*PrefetchStatus)
	log.Printf("Image cache initialized - MaxSize: %d, TTL: %v", imageCache.maxSize, imageCache.ttl)
}

// キャッシュキー生成
func generateCacheKey(archivePath, imageName string) string {
	data := fmt.Sprintf("%s:%s", archivePath, imageName)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// キャッシュから画像取得
func (ic *ImageCache) Get(key string) ([]byte, bool) {
	ic.mutex.RLock()
	defer ic.mutex.RUnlock()
	
	entry, exists := ic.cache[key]
	if !exists {
		return nil, false
	}
	
	// TTL チェック
	if time.Since(entry.Timestamp) > ic.ttl {
		delete(ic.cache, key)
		return nil, false
	}
	
	return entry.Data, true
}

// キャッシュに画像保存
func (ic *ImageCache) Set(key string, data []byte) {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()
	
	// キャッシュサイズ制限チェック
	if len(ic.cache) >= ic.maxSize {
		// 古いエントリを削除（簡単なLRU）
		var oldestKey string
		var oldestTime time.Time = time.Now()
		
		for k, v := range ic.cache {
			if v.Timestamp.Before(oldestTime) {
				oldestTime = v.Timestamp
				oldestKey = k
			}
		}
		
		if oldestKey != "" {
			delete(ic.cache, oldestKey)
		}
	}
	
	ic.cache[key] = &CacheEntry{
		Data:      data,
		Timestamp: time.Now(),
	}
}

// キャッシュクリーンアップ（期限切れエントリ削除）
func (ic *ImageCache) Cleanup() {
	ic.mutex.Lock()
	defer ic.mutex.Unlock()
	
	now := time.Now()
	for key, entry := range ic.cache {
		if now.Sub(entry.Timestamp) > ic.ttl {
			delete(ic.cache, key)
		}
	}
}

func setupRoutes(r *gin.Engine) {
	// HTMLテンプレートの設定
	r.LoadHTMLGlob("templates/*")
	
	// API エンドポイント
	api := r.Group("/api/v1")
	{
		api.GET("/health", healthCheck)
		api.GET("/directories", listDirectories)
		api.GET("/files/*path", listFiles)
		api.GET("/image/*path", serveImage)         // 新機能: 画像配信
		api.GET("/archive/*path", extractArchive)   // 新機能: アーカイブ展開
		api.GET("/archive-image/*path", serveArchiveImage) // 新機能: アーカイブ内画像配信
		api.GET("/prefetch/*path", prefetchImages) // 新機能: 画像プリフェッチ
		api.GET("/cache-status", getCacheStatus) // 新機能: キャッシュ状況確認
		api.GET("/prefetch-status/*path", getPrefetchStatus) // 新機能: プリフェッチ状況確認
		api.GET("/thumbnail/*path", serveThumbnail) // 新機能: サムネイル
	}
	
	// フロントエンドページ
	r.GET("/", indexPage)
	r.GET("/viewer/*path", viewerPage)
}

// インデックスページ
func indexPage(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Manga Server - 軽量Web漫画リーダー",
	})
}

// ビューアページ
func viewerPage(c *gin.Context) {
	path := c.Param("path")
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}
	
	// パスをデコード
	decodedPath, err := url.QueryUnescape(path)
	if err != nil {
		decodedPath = path
	}
	
	c.HTML(http.StatusOK, "viewer.html", gin.H{
		"Title": decodedPath,
		"Path":  path,
	})
}

// ヘルスチェック
func healthCheck(c *gin.Context) {
	// ソースパスの存在確認
	if _, err := os.Stat(config.Manga.SourcePath); os.IsNotExist(err) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  "Source path not accessible: " + config.Manga.SourcePath,
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"status":      "healthy",
		"source_path": config.Manga.SourcePath,
		"phase":       "1",
	})
}

// ディレクトリ一覧取得
func listDirectories(c *gin.Context) {
	dirs, err := scanDirectories(config.Manga.SourcePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to scan directories: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"directories": dirs,
		"count":       len(dirs),
		"base_path":   config.Manga.SourcePath,
	})
}

// ファイル一覧取得
func listFiles(c *gin.Context) {
	requestPath := c.Param("path")
	
	// URLデコード処理
	decodedPath, err := url.QueryUnescape(requestPath)
	if err != nil {
		decodedPath = requestPath
	}
	
	// 先頭のスラッシュを削除
	if strings.HasPrefix(decodedPath, "/") {
		decodedPath = decodedPath[1:]
	}
	
	fullPath := filepath.Join(config.Manga.SourcePath, decodedPath)
	log.Printf("Listing files: %s -> %s -> %s", requestPath, decodedPath, fullPath)
	
	files, err := scanFiles(fullPath)
	if err != nil {
		log.Printf("Failed to scan files in: %s (error: %v)", fullPath, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to scan files: " + err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"files":     files,
		"count":     len(files),
		"path":      requestPath,
		"full_path": fullPath,
	})
}

// ディレクトリスキャン機能
func scanDirectories(basePath string) ([]FileInfo, error) {
	var directories []FileInfo
	
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, err
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			
			directories = append(directories, FileInfo{
				Name:      entry.Name(),
				Path:      entry.Name(),
				IsDir:     true,
				Size:      info.Size(),
				Extension: "",
			})
		}
	}
	
	return directories, nil
}

// ファイルスキャン機能
func scanFiles(dirPath string) ([]FileInfo, error) {
	var files []FileInfo
	
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	
	// サポートするファイル形式
	supportedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
		".zip":  true,
		".rar":  true,
		".cbr":  true,
		".cbz":  true,
	}
	
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		
		// ディレクトリまたはサポートファイルのみ
		if entry.IsDir() || supportedExts[ext] {
			files = append(files, FileInfo{
				Name:      entry.Name(),
				Path:      entry.Name(),
				IsDir:     entry.IsDir(),
				Size:      info.Size(),
				Extension: ext,
			})
		}
	}
	
	return files, nil
}

// CORS ミドルウェア
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// 画像配信機能
func serveImage(c *gin.Context) {
	requestPath := c.Param("path")
	
	// URLデコード処理（+をスペースに変換）
	decodedPath, err := url.QueryUnescape(requestPath)
	if err != nil {
		// デコードに失敗した場合は元のパスを使用
		decodedPath = requestPath
	}
	
	// 先頭のスラッシュを削除
	if strings.HasPrefix(decodedPath, "/") {
		decodedPath = decodedPath[1:]
	}
	
	fullPath := filepath.Join(config.Manga.SourcePath, decodedPath)
	log.Printf("Serving image: %s -> %s -> %s", requestPath, decodedPath, fullPath)
	
	// リサイズパラメータ
	width := c.DefaultQuery("width", "0")
	height := c.DefaultQuery("height", "0")
	quality := c.DefaultQuery("quality", "85")
	
	// ファイル存在確認
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		log.Printf("Image not found: %s", fullPath)
		c.JSON(http.StatusNotFound, gin.H{"error": "Image not found", "path": fullPath})
		return
	}
	
	// 画像ファイルかチェック
	ext := strings.ToLower(filepath.Ext(fullPath))
	if !isImageFile(ext) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not an image file"})
		return
	}
	
	// リサイズが必要かチェック
	w, _ := strconv.Atoi(width)
	h, _ := strconv.Atoi(height)
	q, _ := strconv.Atoi(quality)
	
	if w > 0 || h > 0 {
		// リサイズして配信
		if err := serveResizedImage(c, fullPath, w, h, q); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	
	// オリジナル画像を配信
	c.File(fullPath)
}

// アーカイブ展開機能
func extractArchive(c *gin.Context) {
	requestPath := c.Param("path")
	
	// URLデコード処理
	decodedPath, err := url.QueryUnescape(requestPath)
	if err != nil {
		decodedPath = requestPath
	}
	
	// 先頭のスラッシュを削除
	if strings.HasPrefix(decodedPath, "/") {
		decodedPath = decodedPath[1:]
	}
	
	fullPath := filepath.Join(config.Manga.SourcePath, decodedPath)
	log.Printf("Extracting archive: %s -> %s -> %s", requestPath, decodedPath, fullPath)
	
	// ファイル存在確認
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		log.Printf("Archive not found: %s", fullPath)
		c.JSON(http.StatusNotFound, gin.H{"error": "Archive not found", "path": fullPath})
		return
	}
	
	ext := strings.ToLower(filepath.Ext(fullPath))
	
	var files []FileInfo
	var archiveErr error
	
	switch ext {
	case ".zip", ".cbz":
		files, archiveErr = extractZipFiles(fullPath)
	case ".rar", ".cbr":
		files, archiveErr = extractRarFiles(fullPath)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported archive format"})
		return
	}
	
	if archiveErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": archiveErr.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"files":        files,
		"count":        len(files),
		"archive_path": requestPath,
		"archive_type": ext,
	})
}

// アーカイブ内画像配信機能
func serveArchiveImage(c *gin.Context) {
	requestPath := c.Param("path")
	
	// URLデコード処理
	decodedPath, err := url.QueryUnescape(requestPath)
	if err != nil {
		decodedPath = requestPath
	}
	
	// 先頭のスラッシュを削除
	if strings.HasPrefix(decodedPath, "/") {
		decodedPath = decodedPath[1:]
	}
	
	// パスからアーカイブファイルパスと画像ファイル名を分離
	parts := strings.Split(decodedPath, "/")
	if len(parts) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid path format"})
		return
	}
	
	// 最後の要素が画像ファイル名、それ以外がアーカイブパス
	imageName := parts[len(parts)-1]
	archivePath := strings.Join(parts[:len(parts)-1], "/")
	
	fullArchivePath := filepath.Join(config.Manga.SourcePath, archivePath)
	log.Printf("Serving archive image: %s -> archive: %s, image: %s", requestPath, fullArchivePath, imageName)
	
	// アーカイブファイル存在確認
	if _, err := os.Stat(fullArchivePath); os.IsNotExist(err) {
		log.Printf("Archive not found: %s", fullArchivePath)
		c.JSON(http.StatusNotFound, gin.H{"error": "Archive not found"})
		return
	}
	
	// キャッシュキー生成
	cacheKey := generateCacheKey(fullArchivePath, imageName)
	
	// キャッシュから画像取得を試行
	var imageData []byte
	
	if cachedData, found := imageCache.Get(cacheKey); found {
		log.Printf("Cache hit for: %s", cacheKey)
		imageData = cachedData
	} else {
		log.Printf("Cache miss for: %s, extracting from archive", cacheKey)
		// アーカイブから画像を抽出
		imageData, err = extractImageFromArchive(fullArchivePath, imageName)
		if err != nil {
			log.Printf("Failed to extract image from archive: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Image not found in archive"})
			return
		}
		
		// キャッシュに保存
		imageCache.Set(cacheKey, imageData)
		log.Printf("Cached image: %s (size: %d bytes)", cacheKey, len(imageData))
	}
	
	// リサイズパラメータ
	width := c.DefaultQuery("width", "0")
	height := c.DefaultQuery("height", "0")
	quality := c.DefaultQuery("quality", "85")
	
	w, _ := strconv.Atoi(width)
	h, _ := strconv.Atoi(height)
	q, _ := strconv.Atoi(quality)
	
	if w > 0 || h > 0 {
		// リサイズして配信
		if err := serveResizedImageFromData(c, imageData, w, h, q); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	
	// 画像形式を判定
	ext := strings.ToLower(filepath.Ext(imageName))
	var contentType string
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	default:
		contentType = "application/octet-stream"
	}
	
	// 画像データを直接配信
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, contentType, imageData)
}

// 画像プリフェッチ機能
func prefetchImages(c *gin.Context) {
	requestPath := c.Param("path")
	
	// URLデコード処理
	decodedPath, err := url.QueryUnescape(requestPath)
	if err != nil {
		decodedPath = requestPath
	}
	
	// 先頭のスラッシュを削除
	if strings.HasPrefix(decodedPath, "/") {
		decodedPath = decodedPath[1:]
	}
	
	// パスからアーカイブファイルパスと画像ファイル名を分離
	parts := strings.Split(decodedPath, "/")
	if len(parts) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid path format"})
		return
	}
	
	// 最後の要素が画像ファイル名、それ以外がアーカイブパス
	currentImageName := parts[len(parts)-1]
	archivePath := strings.Join(parts[:len(parts)-1], "/")
	
	fullArchivePath := filepath.Join(config.Manga.SourcePath, archivePath)
	
	// アーカイブファイル存在確認
	if _, err := os.Stat(fullArchivePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Archive not found"})
		return
	}
	
	// アーカイブ内のファイル一覧を取得
	ext := strings.ToLower(filepath.Ext(fullArchivePath))
	var files []FileInfo
	
	switch ext {
	case ".zip", ".cbz":
		files, err = extractZipFiles(fullArchivePath)
	case ".rar", ".cbr":
		files, err = extractRarFiles(fullArchivePath)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported archive format"})
		return
	}
	
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 現在の画像のインデックスを見つける
	currentIndex := -1
	for i, file := range files {
		if file.Name == currentImageName {
			currentIndex = i
			break
		}
	}
	
	if currentIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Current image not found"})
		return
	}
	
	// プリフェッチ状況を初期化（既に進行中でない場合のみ）
	statusKey := fullArchivePath
	prefetchMutex.Lock()
	if status, exists := prefetchStatus[statusKey]; exists && status.InProgress {
		// 既にプリフェッチが進行中の場合は何もしない
		prefetchMutex.Unlock()
		c.JSON(http.StatusOK, gin.H{
			"message": "Prefetch already in progress",
			"status":  status,
		})
		return
	}
	
	prefetchStatus[statusKey] = &PrefetchStatus{
		ArchivePath: fullArchivePath,
		TotalImages: len(files),
		Prefetched:  0,
		InProgress:  true,
		StartTime:   time.Now(),
	}
	prefetchMutex.Unlock()

	// レスポンスを即座に返す
	c.JSON(http.StatusOK, gin.H{
		"message": "Prefetch started",
		"archive": fullArchivePath,
		"current_image": currentImageName,
		"total_images": len(files),
	})

	// 次の100枚の画像をバックグラウンドでプリフェッチ
	go func() {
		defer func() {
			// プリフェッチ完了時に状況を更新
			prefetchMutex.Lock()
			if status, exists := prefetchStatus[statusKey]; exists {
				status.InProgress = false
			}
			prefetchMutex.Unlock()
			}()

	prefetchCount := config.Prefetch.Count
	prefetched := 0
	
	log.Printf("Starting prefetch for %s: current image %s (index %d), will prefetch next %d images", 
		fullArchivePath, currentImageName, currentIndex, prefetchCount)
		
		for i := 1; i <= prefetchCount && currentIndex+i < len(files); i++ {
			nextImage := files[currentIndex+i]
			cacheKey := generateCacheKey(fullArchivePath, nextImage.Name)
			
			// すでにキャッシュされているかチェック
			if _, found := imageCache.Get(cacheKey); !found {
				// キャッシュされていない場合のみ抽出
				imageData, err := extractImageFromArchive(fullArchivePath, nextImage.Name)
				if err == nil {
					imageCache.Set(cacheKey, imageData)
					prefetched++
					log.Printf("Prefetched image: %s (%d/%d)", nextImage.Name, prefetched, prefetchCount)
				} else {
					log.Printf("Failed to prefetch image: %s, error: %v", nextImage.Name, err)
				}
			} else {
				prefetched++
				log.Printf("Image already cached: %s (%d/%d)", nextImage.Name, prefetched, prefetchCount)
			}
			
			// プリフェッチ状況を更新
			prefetchMutex.Lock()
			if status, exists := prefetchStatus[statusKey]; exists {
				status.Prefetched = prefetched
			}
			prefetchMutex.Unlock()
		}
		
		log.Printf("Prefetch completed for %s: %d images cached", fullArchivePath, prefetched)
	}()
	
	c.JSON(http.StatusOK, gin.H{
		"status": "prefetch started",
		"current_index": currentIndex,
		"total_files": len(files),
	})
}

// キャッシュ状況確認API
func getCacheStatus(c *gin.Context) {
	imageCache.mutex.RLock()
	defer imageCache.mutex.RUnlock()
	
	totalSize := int64(0)
	expiredCount := 0
	now := time.Now()
	
	for _, entry := range imageCache.cache {
		totalSize += int64(len(entry.Data))
		if now.Sub(entry.Timestamp) > imageCache.ttl {
			expiredCount++
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"cache_entries": len(imageCache.cache),
		"max_size": imageCache.maxSize,
		"total_memory_bytes": totalSize,
		"total_memory_mb": float64(totalSize) / 1024 / 1024,
		"expired_entries": expiredCount,
		"ttl_minutes": int(imageCache.ttl.Minutes()),
		"cache_hit_ratio": calculateCacheHitRatio(),
	})
}

// キャッシュヒット率計算（簡易版）
func calculateCacheHitRatio() float64 {
	// 実際の実装では、ヒット/ミスのカウンターを追加する必要があります
	// ここでは簡易的にキャッシュエントリ数から推定
	if imageCache.maxSize == 0 {
		return 0.0
	}
	return float64(len(imageCache.cache)) / float64(imageCache.maxSize) * 100
}

// プリフェッチ状況確認API
func getPrefetchStatus(c *gin.Context) {
	requestPath := c.Param("path")
	
	// URLデコード処理
	decodedPath, err := url.QueryUnescape(requestPath)
	if err != nil {
		decodedPath = requestPath
	}
	
	// 先頭のスラッシュを削除
	if strings.HasPrefix(decodedPath, "/") {
		decodedPath = decodedPath[1:]
	}
	
	fullArchivePath := filepath.Join(config.Manga.SourcePath, decodedPath)
	
	prefetchMutex.RLock()
	status, exists := prefetchStatus[fullArchivePath]
	prefetchMutex.RUnlock()
	
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "No prefetch status found for this archive",
			"archive_path": fullArchivePath,
		})
		return
	}
	
	// 進行率を計算
	progressPercent := 0.0
	if status.TotalImages > 0 {
		progressPercent = float64(status.Prefetched) / float64(status.TotalImages) * 100
	}
	
	c.JSON(http.StatusOK, gin.H{
		"archive_path": status.ArchivePath,
		"total_images": status.TotalImages,
		"prefetched": status.Prefetched,
		"in_progress": status.InProgress,
		"progress_percent": progressPercent,
		"start_time": status.StartTime,
		"elapsed_seconds": time.Since(status.StartTime).Seconds(),
	})
}

// サムネイル生成機能
func serveThumbnail(c *gin.Context) {
	requestPath := c.Param("path")
	
	// URLデコード処理
	decodedPath, err := url.QueryUnescape(requestPath)
	if err != nil {
		decodedPath = requestPath
	}
	
	// 先頭のスラッシュを削除
	if strings.HasPrefix(decodedPath, "/") {
		decodedPath = decodedPath[1:]
	}
	
	fullPath := filepath.Join(config.Manga.SourcePath, decodedPath)
	log.Printf("Generating thumbnail: %s -> %s -> %s", requestPath, decodedPath, fullPath)
	
	size := c.DefaultQuery("size", "200")
	thumbnailSize, _ := strconv.Atoi(size)
	if thumbnailSize <= 0 || thumbnailSize > 500 {
		thumbnailSize = 200
	}
	
	// ディレクトリの場合は最初の画像ファイルを探す
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		firstImage, err := findFirstImage(fullPath)
		if err != nil {
			log.Printf("No image found in directory: %s", fullPath)
			c.JSON(http.StatusNotFound, gin.H{"error": "No image found in directory"})
			return
		}
		fullPath = firstImage
		log.Printf("Found first image: %s", fullPath)
	}
	
	// アーカイブファイルの場合は最初の画像を抽出
	ext := strings.ToLower(filepath.Ext(fullPath))
	if isArchiveFile(ext) {
		log.Printf("Extracting thumbnail from archive: %s", fullPath)
		firstImage, err := extractFirstImageFromArchive(fullPath)
		if err != nil {
			log.Printf("Failed to extract image from archive: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		
		// 一時ファイルから読み込み
		if err := serveResizedImageFromData(c, firstImage, thumbnailSize, thumbnailSize, 85); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	
	// 通常の画像ファイル
	if !isImageFile(ext) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not an image file"})
		return
	}
	
	log.Printf("Generating thumbnail for image: %s", fullPath)
	if err := serveResizedImage(c, fullPath, thumbnailSize, thumbnailSize, 85); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}

// ヘルパー関数: 画像ファイル判定
func isImageFile(ext string) bool {
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	}
	return imageExts[ext]
}

// ヘルパー関数: アーカイブファイル判定
func isArchiveFile(ext string) bool {
	archiveExts := map[string]bool{
		".zip": true, ".cbz": true, ".rar": true, ".cbr": true,
	}
	return archiveExts[ext]
}

// リサイズした画像を配信
func serveResizedImage(c *gin.Context, imagePath string, width, height, quality int) error {
	// 画像を読み込み
	img, err := imaging.Open(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open image: %v", err)
	}
	
	// リサイズ処理
	if width > 0 && height > 0 {
		img = imaging.Fit(img, width, height, imaging.Lanczos)
	} else if width > 0 {
		img = imaging.Resize(img, width, 0, imaging.Lanczos)
	} else if height > 0 {
		img = imaging.Resize(img, 0, height, imaging.Lanczos)
	}
	
	// Content-Type設定
	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "public, max-age=3600")
	
	// JPEG形式で出力
	return imaging.Encode(c.Writer, img, imaging.JPEG, imaging.JPEGQuality(quality))
}

// データから画像をリサイズして配信
func serveResizedImageFromData(c *gin.Context, imageData []byte, width, height, quality int) error {
	// バイトデータから画像デコード
	img, err := imaging.Decode(strings.NewReader(string(imageData)))
	if err != nil {
		return fmt.Errorf("failed to decode image: %v", err)
	}
	
	// リサイズ処理
	if width > 0 && height > 0 {
		img = imaging.Fit(img, width, height, imaging.Lanczos)
	} else if width > 0 {
		img = imaging.Resize(img, width, 0, imaging.Lanczos)
	} else if height > 0 {
		img = imaging.Resize(img, 0, height, imaging.Lanczos)
	}
	
	// Content-Type設定
	c.Header("Content-Type", "image/jpeg")
	c.Header("Cache-Control", "public, max-age=3600")
	
	// JPEG形式で出力
	return imaging.Encode(c.Writer, img, imaging.JPEG, imaging.JPEGQuality(quality))
}

// ZIPファイルの内容一覧
func extractZipFiles(zipPath string) ([]FileInfo, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	
	var files []FileInfo
	
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		
		ext := strings.ToLower(filepath.Ext(file.Name))
		if isImageFile(ext) {
			files = append(files, FileInfo{
				Name:      filepath.Base(file.Name),
				Path:      file.Name,
				IsDir:     false,
				Size:      int64(file.UncompressedSize64),
				Extension: ext,
			})
		}
	}
	
	return files, nil
}

// RARファイルの内容一覧
func extractRarFiles(rarPath string) ([]FileInfo, error) {
	file, err := os.Open(rarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	reader, err := rardecode.NewReader(file, "")
	if err != nil {
		return nil, err
	}
	
	var files []FileInfo
	
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		
		if header.IsDir {
			continue
		}
		
		ext := strings.ToLower(filepath.Ext(header.Name))
		if isImageFile(ext) {
			files = append(files, FileInfo{
				Name:      filepath.Base(header.Name),
				Path:      header.Name,
				IsDir:     false,
				Size:      header.UnPackedSize,
				Extension: ext,
			})
		}
	}
	
	return files, nil
}

// ディレクトリ内の最初の画像ファイルを探す
func findFirstImage(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}
	
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if isImageFile(ext) {
			return filepath.Join(dirPath, entry.Name()), nil
		}
	}
	
	return "", fmt.Errorf("no image file found")
}

// アーカイブから最初の画像を抽出
func extractFirstImageFromArchive(archivePath string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))
	
	switch ext {
	case ".zip", ".cbz":
		return extractFirstImageFromZip(archivePath)
	case ".rar", ".cbr":
		return extractFirstImageFromRar(archivePath)
	default:
		return nil, fmt.Errorf("unsupported archive format")
	}
}

// ZIPから最初の画像を抽出
func extractFirstImageFromZip(zipPath string) ([]byte, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		
		ext := strings.ToLower(filepath.Ext(file.Name))
		if isImageFile(ext) {
			rc, err := file.Open()
			if err != nil {
				continue
			}
			defer rc.Close()
			
			data, err := io.ReadAll(rc)
			if err != nil {
				continue
			}
			
			return data, nil
		}
	}
	
	return nil, fmt.Errorf("no image found in archive")
}

// RARから最初の画像を抽出
func extractFirstImageFromRar(rarPath string) ([]byte, error) {
	file, err := os.Open(rarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	reader, err := rardecode.NewReader(file, "")
	if err != nil {
		return nil, err
	}
	
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		
		if header.IsDir {
			continue
		}
		
		ext := strings.ToLower(filepath.Ext(header.Name))
		if isImageFile(ext) {
			data, err := io.ReadAll(reader)
			if err != nil {
				continue
			}
			
			return data, nil
		}
	}
	
	return nil, fmt.Errorf("no image found in archive")
}

// アーカイブから指定画像を抽出
func extractImageFromArchive(archivePath, imageName string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(archivePath))
	
	switch ext {
	case ".zip", ".cbz":
		return extractSpecificImageFromZip(archivePath, imageName)
	case ".rar", ".cbr":
		return extractSpecificImageFromRar(archivePath, imageName)
	default:
		return nil, fmt.Errorf("unsupported archive format")
	}
}

// ZIPから指定画像を抽出
func extractSpecificImageFromZip(zipPath, imageName string) ([]byte, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	
	for _, file := range reader.File {
		if file.Name == imageName {
			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			
			return io.ReadAll(rc)
		}
	}
	
	return nil, fmt.Errorf("image not found in ZIP: %s", imageName)
}

// RARから指定画像を抽出
func extractSpecificImageFromRar(rarPath, imageName string) ([]byte, error) {
	file, err := os.Open(rarPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	
	reader, err := rardecode.NewReader(file, "")
	if err != nil {
		return nil, err
	}
	
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		
		if header.Name == imageName {
			return io.ReadAll(reader)
		}
	}
	
	return nil, fmt.Errorf("image not found in RAR: %s", imageName)
} 