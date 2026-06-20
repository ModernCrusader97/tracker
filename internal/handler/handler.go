package handler

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"tracker/internal/auth"
	"tracker/internal/model"
	"tracker/internal/service"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *service.ItemService
}

func New(svc *service.ItemService) *Handler { return &Handler{svc: svc} }

func (h *Handler) jwtSecret() string {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return s
	}
	return "tracker-secret-key"
}

func (h *Handler) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		userID, err := auth.ParseJWT(token, h.jwtSecret())
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Set("user_id", userID)
		c.Next()
	}
}

func userID(c *gin.Context) int64 {
	v, _ := c.Get("user_id")
	id, _ := v.(int64)
	return id
}

func (h *Handler) Register(r *gin.Engine) {
	api := r.Group("/api")

	// Public
	api.POST("/auth/telegram", h.TelegramLogin)
	api.GET("/auth/bot-login/init", h.BotLoginInit)
	api.GET("/auth/bot-login/poll", h.BotLoginPoll)
	api.GET("/settings/telegram/bot-info", h.GetBotInfo)
	api.GET("/items/:id/done-quick", h.DoneQuick)
	api.GET("/items/:id/snooze", h.SnoozeItem)

	// Protected
	protected := api.Group("/", h.authMiddleware())
	protected.GET("/me", h.Me)
	protected.GET("/items", h.List)
	protected.POST("/items", h.Create)
	protected.PUT("/items/:id", h.Update)
	protected.DELETE("/items/:id", h.Delete)
	protected.POST("/items/:id/check", h.CheckHabit)
	protected.GET("/items/:id/heatmap", h.Heatmap)
	protected.GET("/settings/telegram", h.GetTelegramSettings)
	protected.PUT("/settings/telegram", h.SaveTelegramSettings)
	protected.POST("/settings/telegram/detect-chat", h.DetectChatID)
	protected.POST("/settings/telegram/test", h.TestTelegram)
	protected.GET("/notifications/poll", h.PollNotifications)
	protected.GET("/stats", h.Stats)
	protected.PUT("/settings/fcm-token", h.SaveFCMToken)
}

func (h *Handler) TelegramLogin(c *gin.Context) {
	var data map[string]string
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data"})
		return
	}
	var botToken string
	h.svc.DB().QueryRow(c.Request.Context(),
		`SELECT value FROM settings WHERE key='telegram_token'`).Scan(&botToken)
	if botToken == "" {
		botToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	}
	if !auth.VerifyTelegram(data, botToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid telegram auth"})
		return
	}
	telegramID, _ := strconv.ParseInt(data["id"], 10, 64)
	user, err := h.svc.FindOrCreateUser(c.Request.Context(), telegramID, data["username"], data["first_name"], data["photo_url"])
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	jwt, err := auth.MakeJWT(user.ID, h.jwtSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": jwt, "user": user})
}

func (h *Handler) Me(c *gin.Context) {
	uid := userID(c)
	row := h.svc.DB().QueryRow(c.Request.Context(),
		`SELECT id, telegram_id, username, first_name, photo_url, created_at FROM users WHERE id=$1`, uid)
	u := &model.User{}
	if err := row.Scan(&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.PhotoURL, &u.CreatedAt); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, u)
}

func (h *Handler) PollNotifications(c *gin.Context) {
	sinceID, _ := strconv.ParseInt(c.Query("since_id"), 10, 64)

	rows, err := h.svc.DB().Query(c.Request.Context(),
		`SELECT id, text, item_id, item_type, sent_at FROM notification_queue WHERE id > $1 ORDER BY id ASC LIMIT 50`,
		sinceID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type Notif struct {
		ID       int64   `json:"id"`
		Text     string  `json:"text"`
		ItemID   *int64  `json:"item_id,omitempty"`
		ItemType *string `json:"item_type,omitempty"`
		SentAt   int64   `json:"sent_at_ms"`
	}
	var results []Notif
	for rows.Next() {
		var n Notif
		var sentAt time.Time
		if err := rows.Scan(&n.ID, &n.Text, &n.ItemID, &n.ItemType, &sentAt); err != nil {
			continue
		}
		n.SentAt = sentAt.UnixMilli()
		results = append(results, n)
	}
	if results == nil {
		results = []Notif{}
	}
	c.JSON(http.StatusOK, results)
}

func (h *Handler) SaveFCMToken(c *gin.Context) {
	var req struct {
		Token string `json:"token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	h.svc.DB().Exec(c.Request.Context(),
		`INSERT INTO settings (key, value) VALUES ('fcm_device_token', $1) ON CONFLICT (key) DO UPDATE SET value=$1`,
		req.Token)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context(), userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *Handler) Create(c *gin.Context) {
	var req model.CreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	it, err := h.svc.Create(c.Request.Context(), &req, userID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, it)
}

func (h *Handler) Update(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req model.UpdateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	it, err := h.svc.Update(c.Request.Context(), id, userID(c), &req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, it)
}

func (h *Handler) CheckHabit(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	it, err := h.svc.CheckHabit(c.Request.Context(), id, userID(c))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, it)
}

func (h *Handler) Delete(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.svc.Delete(c.Request.Context(), id, userID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *Handler) Heatmap(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	dates, _ := h.svc.HeatmapData(c.Request.Context(), id)
	c.JSON(http.StatusOK, dates)
}

// ─── Telegram Settings ───────────────────────────────────────────

func (h *Handler) GetTelegramSettings(c *gin.Context) {
	db := h.svc.DB()
	ctx := c.Request.Context()
	keys := []string{"telegram_token", "telegram_chat_id", "dnd_start", "dnd_end", "briefing_morning", "briefing_evening", "notification_mode"}
	vals := map[string]string{}
	for _, k := range keys {
		var v string
		db.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, k).Scan(&v)
		vals[k] = v
	}
	masked := ""
	if t := vals["telegram_token"]; len(t) > 10 {
		masked = t[:10] + "..." + t[len(t)-4:]
	}
	notifMode := vals["notification_mode"]
	if notifMode == "" {
		notifMode = "both"
	}
	c.JSON(http.StatusOK, gin.H{
		"token_set":         vals["telegram_token"] != "",
		"token_masked":      masked,
		"chat_id":           vals["telegram_chat_id"],
		"dnd_start":         vals["dnd_start"],
		"dnd_end":           vals["dnd_end"],
		"briefing_morning":  vals["briefing_morning"],
		"briefing_evening":  vals["briefing_evening"],
		"notification_mode": notifMode,
	})
}

func (h *Handler) SaveTelegramSettings(c *gin.Context) {
	var req struct {
		Token            string `json:"token"`
		ChatID           string `json:"chat_id"`
		DndStart         string `json:"dnd_start"`
		DndEnd           string `json:"dnd_end"`
		BriefingMorning  string `json:"briefing_morning"`
		BriefingEvening  string `json:"briefing_evening"`
		NotificationMode string `json:"notification_mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	db := h.svc.DB()
	ctx := c.Request.Context()
	upsert := func(key, val string) {
		db.Exec(ctx, `INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value=$2`, key, val)
	}
	if req.Token != "" { upsert("telegram_token", req.Token) }
	if req.ChatID != "" { upsert("telegram_chat_id", req.ChatID) }
	upsert("dnd_start", req.DndStart)
	upsert("dnd_end", req.DndEnd)
	upsert("briefing_morning", req.BriefingMorning)
	upsert("briefing_evening", req.BriefingEvening)
	if req.NotificationMode != "" { upsert("notification_mode", req.NotificationMode) }
	c.JSON(http.StatusOK, gin.H{"saved": true})
}

func (h *Handler) DetectChatID(c *gin.Context) {
	ctx := c.Request.Context()
	db := h.svc.DB()

	// First check if poller already captured a chat_id from an incoming message
	var detected string
	db.QueryRow(ctx, `SELECT value FROM settings WHERE key='detected_chat_id'`).Scan(&detected)
	if detected != "" {
		c.JSON(http.StatusOK, gin.H{"chat_id": detected})
		return
	}

	// Fallback: try getUpdates directly with the provided/saved token
	var req struct{ Token string `json:"token"` }
	c.ShouldBindJSON(&req)
	if req.Token == "" {
		db.QueryRow(ctx, `SELECT value FROM settings WHERE key='telegram_token'`).Scan(&req.Token)
	}
	if req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "봇 토큰을 먼저 저장해주세요"})
		return
	}

	body, _ := json.Marshal(map[string]any{"timeout": 5, "limit": 5, "allowed_updates": []string{"message"}})
	resp, err := http.Post(fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", req.Token),
		"application/json", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Telegram API 연결 실패"})
		return
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool `json:"ok"`
		Result []struct {
			Message *struct {
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if !result.OK || len(result.Result) == 0 || result.Result[0].Message == nil {
		c.JSON(http.StatusOK, gin.H{"chat_id": "", "hint": "봇에게 메시지를 보낸 후 다시 시도해주세요 (예: /start)"})
		return
	}
	chatID := fmt.Sprintf("%d", result.Result[0].Message.Chat.ID)
	// Save for future
	db.Exec(ctx, `INSERT INTO settings (key, value) VALUES ('detected_chat_id', $1) ON CONFLICT (key) DO UPDATE SET value=$1`, chatID)
	c.JSON(http.StatusOK, gin.H{"chat_id": chatID})
}

func (h *Handler) TestTelegram(c *gin.Context) {
	ctx := c.Request.Context()
	db := h.svc.DB()
	var token, chatID string
	db.QueryRow(ctx, `SELECT value FROM settings WHERE key='telegram_token'`).Scan(&token)
	db.QueryRow(ctx, `SELECT value FROM settings WHERE key='telegram_chat_id'`).Scan(&chatID)
	if token == "" || chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "봇 토큰과 채팅 ID를 저장해주세요"})
		return
	}
	body, _ := json.Marshal(map[string]any{
		"chat_id": chatID, "text": "✅ 트래커 앱 텔레그램 연결 완료!", "parse_mode": "HTML",
	})
	resp, err := http.Post(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token),
		"application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != 200 {
		c.JSON(http.StatusBadGateway, gin.H{"error": "메시지 전송 실패. 토큰과 채팅 ID를 확인해주세요"})
		return
	}
	resp.Body.Close()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) GetBotInfo(c *gin.Context) {
	ctx := c.Request.Context()
	var token string
	h.svc.DB().QueryRow(ctx, `SELECT value FROM settings WHERE key='telegram_token'`).Scan(&token)
	if token == "" {
		token = os.Getenv("TELEGRAM_BOT_TOKEN")
	}
	if token == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false})
		return
	}

	tgGet := func(method string, payload map[string]any) (map[string]any, error) {
		body, _ := json.Marshal(payload)
		resp, err := http.Post(fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method), "application/json", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var out map[string]any
		json.NewDecoder(resp.Body).Decode(&out)
		return out, nil
	}

	meRes, err := tgGet("getMe", map[string]any{})
	if err != nil || meRes["ok"] != true {
		c.JSON(http.StatusOK, gin.H{"ok": false})
		return
	}
	me, ok := meRes["result"].(map[string]any)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"ok": false})
		return
	}
	botIDf, _ := me["id"].(float64)
	botID := int64(botIDf)
	username, _ := me["username"].(string)
	name, _ := me["first_name"].(string)

	// fetch profile photo
	photoURL := ""
	photosRes, err := tgGet("getUserProfilePhotos", map[string]any{"user_id": botID, "limit": 1})
	if err == nil && photosRes["ok"] == true {
		if photos, ok := photosRes["result"].(map[string]any); ok {
			totalF, _ := photos["total_count"].(float64)
			if int(totalF) > 0 {
				if sets, ok := photos["photos"].([]any); ok && len(sets) > 0 {
					if photoSet, ok := sets[0].([]any); ok && len(photoSet) > 0 {
						if fileInfo, ok := photoSet[0].(map[string]any); ok {
							fileID, _ := fileInfo["file_id"].(string)
							fileRes, err2 := tgGet("getFile", map[string]any{"file_id": fileID})
							if err2 == nil && fileRes["ok"] == true {
								if fileResult, ok := fileRes["result"].(map[string]any); ok {
									filePath, _ := fileResult["file_path"].(string)
									photoURL = fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", token, filePath)
								}
							}
						}
					}
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"ok":        true,
		"username":  username,
		"name":      name,
		"photo_url": photoURL,
	})
}

// SnoozeItem is called when user taps the snooze URL button in Telegram notification.
func (h *Handler) SnoozeItem(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	hoursStr := c.Query("hours")
	hours := 1
	if n, err := strconv.Atoi(hoursStr); err == nil && n > 0 {
		hours = n
	}
	until := time.Now().Add(time.Duration(hours) * time.Hour)
	h.svc.DB().Exec(c.Request.Context(), `UPDATE items SET snoozed_until=$1 WHERE id=$2`, until, id)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!doctype html>
<html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>body{background:#0f1117;color:#e8eaf0;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;font-size:20px;text-align:center}</style>
</head><body>⏸️ `+strconv.Itoa(hours)+`시간 동안 알림을 끕니다</body></html>`))
}

// DoneQuick is called when user taps the URL button in Telegram notification.
func (h *Handler) DoneQuick(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	itemType := c.Query("type")

	var msg string
	if itemType == "habit" {
		it, err := h.svc.CheckHabit(c.Request.Context(), id, 0)
		if err != nil {
			msg = "이미 오늘 체크했어요 ✓"
		} else {
			msg = "오늘 체크 완료! 🎉"
			if it.Streak > 1 {
				msg += " " + strconv.Itoa(it.Streak) + "일 연속 🔥"
			}
		}
	} else {
		done := true
		h.svc.Update(c.Request.Context(), id, 0, &model.UpdateItemRequest{Done: &done})
		msg = "완료 처리됐어요! ✅"
	}

	// Simple HTML response shown in Telegram's webview
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!doctype html>
<html><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>body{background:#0f1117;color:#e8eaf0;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;font-size:20px;text-align:center}</style>
</head><body>`+msg+`</body></html>`))
}

func (h *Handler) Stats(c *gin.Context) {
	stats, err := h.svc.Stats(c.Request.Context(), userID(c))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, stats)
}

// BotLoginInit generates a one-time login code and returns a t.me deep link.
func (h *Handler) BotLoginInit(c *gin.Context) {
	ctx := c.Request.Context()
	b := make([]byte, 8)
	rand.Read(b)
	code := hex.EncodeToString(b)
	h.svc.DB().Exec(ctx,
		`INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value=$2`,
		"bot_login_code:"+code, "pending:"+fmt.Sprintf("%d", time.Now().Unix()),
	)
	var botUsername string
	var token string
	h.svc.DB().QueryRow(ctx, `SELECT value FROM settings WHERE key='telegram_token'`).Scan(&token)
	if token != "" {
		body, _ := json.Marshal(map[string]any{})
		resp, err := http.Post(fmt.Sprintf("https://api.telegram.org/bot%s/getMe", token), "application/json", bytes.NewReader(body))
		if err == nil {
			var me struct{ Result struct{ Username string `json:"username"` } `json:"result"` }
			json.NewDecoder(resp.Body).Decode(&me)
			resp.Body.Close()
			botUsername = me.Result.Username
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"code":     code,
		"deep_link": fmt.Sprintf("https://t.me/%s?start=login_%s", botUsername, code),
		"bot":      botUsername,
	})
}

// BotLoginPoll checks if a login code has been confirmed by the bot.
func (h *Handler) BotLoginPoll(c *gin.Context) {
	ctx := c.Request.Context()
	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code"})
		return
	}
	var val string
	h.svc.DB().QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, "bot_login_code:"+code).Scan(&val)
	if !strings.HasPrefix(val, "uid:") {
		c.JSON(http.StatusOK, gin.H{"ready": false})
		return
	}
	uid, _ := strconv.ParseInt(strings.TrimPrefix(val, "uid:"), 10, 64)
	jwtToken, err := auth.MakeJWT(uid, h.jwtSecret())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// Clean up the code
	h.svc.DB().Exec(ctx, `DELETE FROM settings WHERE key=$1`, "bot_login_code:"+code)
	c.JSON(http.StatusOK, gin.H{"ready": true, "token": jwtToken})
}
