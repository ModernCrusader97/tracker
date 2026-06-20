package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"tracker/internal/db"
	"tracker/internal/handler"
	"tracker/internal/notifier"
	"tracker/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/tracker?sslmode=disable"
	}

	pool, err := db.New(dbURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	svc := service.New(pool)
	h := handler.New(svc)

	serverURL := os.Getenv("SERVER_PUBLIC_URL")
	if serverURL == "" {
		serverURL = "http://46.224.110.173:8082"
	}
	// Seed initial telegram config from env if DB not yet configured
	seedTelegramFromEnv(pool)

	notif := notifier.New(pool, serverURL)
	go notif.Run(context.Background())
	go notif.RunCallbackPoller(context.Background())

	r := gin.Default()
	r.Use(cors())
	h.Register(r)

	// SPA static files
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = filepath.Join("..", "tracker-web", "dist")
	}
	if _, err := os.Stat(staticDir); err == nil {
		r.Static("/assets", filepath.Join(staticDir, "assets"))
		r.StaticFile("/favicon.ico", filepath.Join(staticDir, "favicon.ico"))
		r.NoRoute(func(c *gin.Context) {
			c.File(filepath.Join(staticDir, "index.html"))
		})
	}

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8082"
	}
	log.Printf("tracker starting on :%s", port)
	srv := &http.Server{Addr: ":" + port, Handler: r}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}

func seedTelegramFromEnv(pool *pgxpool.Pool) {
	ctx := context.Background()
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if token != "" {
		pool.Exec(ctx, `INSERT INTO settings (key, value) VALUES ('telegram_token', $1) ON CONFLICT (key) DO NOTHING`, token)
	}
	if chatID != "" {
		pool.Exec(ctx, `INSERT INTO settings (key, value) VALUES ('telegram_chat_id', $1) ON CONFLICT (key) DO NOTHING`, chatID)
	}
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
