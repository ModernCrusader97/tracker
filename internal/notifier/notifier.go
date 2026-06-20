package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"tracker/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2/google"
)

type Notifier struct {
	db        *pgxpool.Pool
	serverURL string
}

func New(db *pgxpool.Pool, serverURL string) *Notifier {
	return &Notifier{db: db, serverURL: serverURL}
}

// creds reads bot token and chat_id from DB settings, falling back to env.
func (n *Notifier) creds(ctx context.Context) (token, chatID string) {
	n.db.QueryRow(ctx, `SELECT value FROM settings WHERE key='telegram_token'`).Scan(&token)
	n.db.QueryRow(ctx, `SELECT value FROM settings WHERE key='telegram_chat_id'`).Scan(&chatID)
	return
}

func tgPost(token, method string, payload map[string]any) (map[string]any, error) {
	body, _ := json.Marshal(payload)
	resp, err := http.Post(
		fmt.Sprintf("https://api.telegram.org/bot%s/%s", token, method),
		"application/json", bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	if ok, _ := result["ok"].(bool); !ok {
		desc, _ := result["description"].(string)
		if desc == "" {
			desc = "unknown telegram error"
		}
		return result, fmt.Errorf("telegram: %s", desc)
	}
	return result, nil
}

// notifMode returns "both", "telegram", or "app"
func (n *Notifier) notifMode(ctx context.Context) string {
	var mode string
	n.db.QueryRow(ctx, `SELECT value FROM settings WHERE key='notification_mode'`).Scan(&mode)
	if mode == "" {
		return "both"
	}
	return mode
}

func (n *Notifier) queueOnly(ctx context.Context, text string, itemID *int64, itemType string) {
	if itemID != nil {
		n.db.Exec(ctx, `INSERT INTO notification_queue (text, item_id, item_type) VALUES ($1, $2, $3)`, text, *itemID, itemType)
	} else {
		n.db.Exec(ctx, `INSERT INTO notification_queue (text) VALUES ($1)`, text)
	}
	n.db.Exec(ctx, `DELETE FROM notification_queue WHERE sent_at < NOW() - INTERVAL '7 days'`)
}

// Send plain text message.
func (n *Notifier) Send(ctx context.Context, text string) error {
	mode := n.notifMode(ctx)
	if mode == "app" {
		n.queueOnly(ctx, text, nil, "")
		n.sendFCM(ctx, "트래커", text)
		return nil
	}
	token, chatID := n.creds(ctx)
	if token == "" || chatID == "" {
		return nil
	}
	_, err := tgPost(token, "sendMessage", map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	})
	if err == nil && mode == "both" {
		n.queueOnly(ctx, text, nil, "")
		n.sendFCM(ctx, "트래커", text)
	}
	return err
}

// SendWithDoneButton sends a message with done + snooze buttons.
func (n *Notifier) SendWithDoneButton(ctx context.Context, text string, itemID int64, itemType string) error {
	mode := n.notifMode(ctx)
	if mode == "app" {
		n.queueOnly(ctx, text, &itemID, itemType)
		n.sendFCM(ctx, "트래커", text)
		return nil
	}
	token, chatID := n.creds(ctx)
	if token == "" || chatID == "" {
		return nil
	}
	btnText := "✅ 완료 체크"
	if itemType == "habit" {
		btnText = "✅ 오늘 체크"
	}
	_, err := tgPost(token, "sendMessage", map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
		"reply_markup": map[string]any{
			"inline_keyboard": [][]map[string]any{
				{
					{"text": btnText, "callback_data": fmt.Sprintf("done:%d:%s", itemID, itemType)},
					{"text": "⏸️ 1시간 알림 끄기", "callback_data": fmt.Sprintf("snooze:%d", itemID)},
				},
			},
		},
	})
	if err == nil && mode == "both" {
		n.queueOnly(ctx, text, &itemID, itemType)
		n.sendFCM(ctx, "트래커", text)
	}
	return err
}

// RunCallbackPoller polls Telegram for callback queries and handles them.
func (n *Notifier) RunCallbackPoller(ctx context.Context) {
	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		token, _ := n.creds(ctx)
		if token == "" {
			time.Sleep(10 * time.Second)
			continue
		}

		result, err := tgPost(token, "getUpdates", map[string]any{
			"offset":          offset,
			"timeout":         25,
			"allowed_updates": []string{"message", "callback_query"},
		})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		updates, _ := result["result"].([]any)
		for _, u := range updates {
			upd, _ := u.(map[string]any)
			updID, _ := upd["update_id"].(float64)
			offset = int64(updID) + 1

			// If it's a regular message, save chat_id and handle bot login
			if msg, ok := upd["message"].(map[string]any); ok {
				if chat, ok := msg["chat"].(map[string]any); ok {
					if chatID, ok := chat["id"].(float64); ok {
						cid := fmt.Sprintf("%d", int64(chatID))
						n.db.Exec(ctx, `INSERT INTO settings (key, value) VALUES ('detected_chat_id', $1) ON CONFLICT (key) DO UPDATE SET value=$1`, cid)

						// Handle /start login_CODE
						text, _ := msg["text"].(string)
						if strings.HasPrefix(text, "/start login_") {
							code := strings.TrimPrefix(text, "/start login_")
							var existing string
							n.db.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, "bot_login_code:"+code).Scan(&existing)
							if strings.HasPrefix(existing, "pending:") {
								// Find or create user for this telegram id
								telegramID := int64(chatID)
								fromUser, _ := msg["from"].(map[string]any)
								username, _ := fromUser["username"].(string)
								firstName, _ := fromUser["first_name"].(string)
								var uid int64
								n.db.QueryRow(ctx, `SELECT id FROM users WHERE telegram_id=$1`, telegramID).Scan(&uid)
								if uid == 0 {
									n.db.QueryRow(ctx,
										`INSERT INTO users (telegram_id, username, first_name) VALUES ($1,$2,$3) ON CONFLICT (telegram_id) DO UPDATE SET username=$2, first_name=$3 RETURNING id`,
										telegramID, username, firstName).Scan(&uid)
								}
								if uid > 0 {
									n.db.Exec(ctx, `UPDATE settings SET value=$1 WHERE key=$2`,
										fmt.Sprintf("uid:%d", uid), "bot_login_code:"+code)
									tgPost(token, "sendMessage", map[string]any{
										"chat_id": int64(chatID),
										"text":    "✅ 로그인 완료! 앱으로 돌아가세요.",
									})
								}
							}
						}
					}
				}
			}

			cq, _ := upd["callback_query"].(map[string]any)
			if cq == nil {
				continue
			}
			cqID, _ := cq["id"].(string)
			data, _ := cq["data"].(string)

			answer := "✅"
			parts := strings.SplitN(data, ":", 3)
			switch parts[0] {
			case "done":
				if len(parts) >= 3 {
					id, _ := strconv.ParseInt(parts[1], 10, 64)
					itemType := parts[2]
					if itemType == "habit" {
						n.db.Exec(ctx, `UPDATE items SET last_checked_at=NOW(), streak=streak+1 WHERE id=$1`, id)
						n.db.Exec(ctx, `INSERT INTO habit_checks (item_id, checked_on) VALUES ($1, CURRENT_DATE) ON CONFLICT DO NOTHING`, id)
						answer = "✅ 오늘 체크 완료!"
					} else {
						n.db.Exec(ctx, `UPDATE items SET done=true, done_at=NOW() WHERE id=$1`, id)
						answer = "✅ 완료 처리됐어요!"
					}
				}
			case "snooze":
				if len(parts) >= 2 {
					id, _ := strconv.ParseInt(parts[1], 10, 64)
					n.db.Exec(ctx, `UPDATE items SET snoozed_until=NOW() + INTERVAL '1 hour' WHERE id=$1`, id)
					answer = "⏸️ 1시간 알림 끔"
				}
			}

			tgPost(token, "answerCallbackQuery", map[string]any{
				"callback_query_id": cqID,
				"text":              answer,
			})
		}
	}
}

// Run starts the notification scheduler.
func (n *Notifier) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.tick(ctx)
		}
	}
}

func (n *Notifier) dndActive(ctx context.Context, now time.Time) bool {
	var startStr, endStr string
	n.db.QueryRow(ctx, `SELECT value FROM settings WHERE key='dnd_start'`).Scan(&startStr)
	n.db.QueryRow(ctx, `SELECT value FROM settings WHERE key='dnd_end'`).Scan(&endStr)
	if startStr == "" || endStr == "" {
		return false
	}
	var start, end int
	fmt.Sscanf(startStr, "%d", &start)
	fmt.Sscanf(endStr, "%d", &end)
	h := now.Hour()
	if start <= end {
		return h >= start && h < end
	}
	// wraps midnight: e.g. 23–7
	return h >= start || h < end
}

var kst = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*60*60)
	}
	return loc
}()

// kstToday returns the start of the current calendar day in KST (00:00:00 KST).
// time.Truncate(24h) computes UTC midnight, not local midnight — do not use it.
func kstToday(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

func stripHTML(s string) string {
	return strings.TrimSpace(htmlTagRe.ReplaceAllString(s, ""))
}

func (n *Notifier) sendFCM(ctx context.Context, title, body string) {
	credsFile := os.Getenv("FIREBASE_CREDENTIALS_FILE")
	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if credsFile == "" || projectID == "" {
		return
	}
	var deviceToken string
	n.db.QueryRow(ctx, `SELECT value FROM settings WHERE key='fcm_device_token'`).Scan(&deviceToken)
	if deviceToken == "" {
		return
	}
	credsJSON, err := os.ReadFile(credsFile)
	if err != nil {
		log.Printf("FCM: cannot read credentials file: %v", err)
		return
	}
	conf, err := google.JWTConfigFromJSON(credsJSON, "https://www.googleapis.com/auth/firebase.messaging")
	if err != nil {
		return
	}
	tok, err := conf.TokenSource(ctx).Token()
	if err != nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"message": map[string]any{
			"token": deviceToken,
			"notification": map[string]string{
				"title": title,
				"body":  stripHTML(body),
			},
			"android": map[string]any{
				"priority": "high",
				"notification": map[string]string{
					"channel_id": "tracker-alerts",
				},
			},
		},
	})
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID),
		bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Printf("FCM send failed: %d", resp.StatusCode)
	}
}

func (n *Notifier) tick(ctx context.Context) {
	now := time.Now().In(kst)
	n.checkBriefings(ctx, now)
	if n.dndActive(ctx, now) {
		return
	}
	n.checkTodos(ctx, now)
	n.checkHabits(ctx, now)
}

func (n *Notifier) checkTodos(ctx context.Context, now time.Time) {
	rows, err := n.db.Query(ctx, `
		SELECT id, title, remind_every_min, due_date FROM items
		WHERE type='todo' AND done=false`)
	if err != nil {
		return
	}
	defer rows.Close()

	type todo struct {
		id             int64
		title          string
		remindEveryMin int
		dueDate        *string
	}
	var todos []todo
	for rows.Next() {
		var t todo
		rows.Scan(&t.id, &t.title, &t.remindEveryMin, &t.dueDate)
		todos = append(todos, t)
	}

	today := kstToday(now)

	for _, t := range todos {
		effectiveMin := t.remindEveryMin
		urgencyMsg := ""

		if t.dueDate != nil && *t.dueDate != "" {
			due, err := time.Parse("2006-01-02", *t.dueDate)
			if err == nil {
				daysLeft := int(due.Sub(today).Hours() / 24)
				switch {
				case daysLeft < 0:
					effectiveMin = 60
					urgencyMsg = fmt.Sprintf("🚨 <b>마감일 %d일 초과!</b>", -daysLeft)
				case daysLeft == 0:
					if effectiveMin == 0 || effectiveMin > 30 {
						effectiveMin = 30
					}
					urgencyMsg = "🔴 <b>오늘 마감!</b>"
				case daysLeft == 1:
					if effectiveMin == 0 || effectiveMin > 120 {
						effectiveMin = 120
					}
					urgencyMsg = "🟡 <b>내일 마감</b>"
				case daysLeft <= 3:
					if effectiveMin == 0 || effectiveMin > 240 {
						effectiveMin = 240
					}
					urgencyMsg = fmt.Sprintf("⚠️ <b>D-%d</b>", daysLeft)
				}
			}
		}

		if effectiveMin == 0 {
			continue
		}

		var snoozedUntil *time.Time
		n.db.QueryRow(ctx, `SELECT snoozed_until FROM items WHERE id=$1`, t.id).Scan(&snoozedUntil)
		if snoozedUntil != nil && now.Before(*snoozedUntil) {
			continue
		}

		var lastSent *time.Time
		n.db.QueryRow(ctx, `SELECT MAX(sent_at) FROM reminder_log WHERE item_id=$1`, t.id).Scan(&lastSent)

		threshold := time.Duration(effectiveMin)*time.Minute - 30*time.Second
		isDue := false
		if lastSent == nil {
			var created time.Time
			n.db.QueryRow(ctx, `SELECT created_at FROM items WHERE id=$1`, t.id).Scan(&created)
			isDue = now.Sub(created) >= threshold
		} else {
			isDue = now.Sub(*lastSent) >= threshold
		}

		if isDue {
			var msg string
			if urgencyMsg != "" {
				msg = fmt.Sprintf("%s\n\"%s\" 아직 안 하셨어요!", urgencyMsg, t.title)
			} else {
				msg = fmt.Sprintf("⏰ <b>할 일 미완료</b>\n\"%s\" 아직 안 하셨어요!", t.title)
			}
			if err := n.SendWithDoneButton(ctx, msg, t.id, "todo"); err == nil {
				n.db.Exec(ctx, `INSERT INTO reminder_log (item_id, remind_type) VALUES ($1, 'main')`, t.id)
				time.Sleep(500 * time.Millisecond)
			} else {
				log.Printf("telegram send error: %v", err)
			}
		}
	}
}

func isScheduledDay(freq model.FreqType, freqDays string, wd time.Weekday) bool {
	if freq != model.FreqDaysOfWeek {
		return true
	}
	for _, part := range strings.Split(freqDays, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && time.Weekday(n) == wd {
			return true
		}
	}
	return false
}

func (n *Notifier) checkHabits(ctx context.Context, now time.Time) {
	today := kstToday(now)

	rows, err := n.db.Query(ctx, `
		SELECT id, title, frequency, freq_days, reminder_hour, reminder_minute, remind_before_min, streak, last_checked_at
		FROM items WHERE type='habit'`)
	if err != nil {
		return
	}
	defer rows.Close()

	type habit struct {
		id              int64
		title           string
		freq            model.FreqType
		freqDays        string
		reminderHour    int
		reminderMinute  int
		remindBeforeMin int
		streak          int
		lastCheckedAt   *time.Time
	}
	var habits []habit
	for rows.Next() {
		var h habit
		rows.Scan(&h.id, &h.title, &h.freq, &h.freqDays, &h.reminderHour, &h.reminderMinute, &h.remindBeforeMin, &h.streak, &h.lastCheckedAt)
		habits = append(habits, h)
	}

	for _, h := range habits {
		if !isScheduledDay(h.freq, h.freqDays, now.Weekday()) {
			continue
		}

		checkedToday := h.lastCheckedAt != nil && h.lastCheckedAt.After(today)
		if checkedToday {
			continue
		}

		var snoozedUntil *time.Time
		n.db.QueryRow(ctx, `SELECT snoozed_until FROM items WHERE id=$1`, h.id).Scan(&snoozedUntil)
		if snoozedUntil != nil && now.Before(*snoozedUntil) {
			continue
		}

		nowMin := now.Hour()*60 + now.Minute()
		mainMin := h.reminderHour*60 + h.reminderMinute

		// pre-reminder
		if h.remindBeforeMin > 0 {
			preMin := mainMin - h.remindBeforeMin
			if preMin < 0 {
				preMin += 24 * 60
			}
			if nowMin >= preMin && nowMin <= preMin+1 {
				var lastPre *time.Time
				n.db.QueryRow(ctx, `SELECT MAX(sent_at) FROM reminder_log WHERE item_id=$1 AND sent_at > $2 AND remind_type='pre'`, h.id, today).Scan(&lastPre)
				if lastPre == nil {
					msg := fmt.Sprintf("⏰ <b>%d분 후 체크 시간!</b>\n\"%s\" 준비하세요!", h.remindBeforeMin, h.title)
					if err := n.SendWithDoneButton(ctx, msg, h.id, "habit"); err == nil {
						n.db.Exec(ctx, `INSERT INTO reminder_log (item_id, remind_type) VALUES ($1, 'pre')`, h.id)
						time.Sleep(500 * time.Millisecond)
					}
				}
			}
		}

		// main reminder
		if nowMin >= mainMin && nowMin <= mainMin+1 {
			var lastMain *time.Time
			n.db.QueryRow(ctx, `SELECT MAX(sent_at) FROM reminder_log WHERE item_id=$1 AND sent_at > $2 AND remind_type='main'`, h.id, today).Scan(&lastMain)
			if lastMain == nil {
				streakMsg := ""
				if h.streak > 0 {
					streakMsg = fmt.Sprintf(" (현재 %d일 연속)", h.streak)
				}
				msg := fmt.Sprintf("🔔 <b>습관 체크 시간!</b>\n\"%s\" 오늘 아직 체크 안 하셨어요%s", h.title, streakMsg)
				if err := n.SendWithDoneButton(ctx, msg, h.id, "habit"); err == nil {
					n.db.Exec(ctx, `INSERT INTO reminder_log (item_id, remind_type) VALUES ($1, 'main')`, h.id)
					time.Sleep(500 * time.Millisecond)
				}
			}
		}
	}
}

func (n *Notifier) settingHour(ctx context.Context, key string) (hour, min int, ok bool) {
	var val string
	n.db.QueryRow(ctx, `SELECT value FROM settings WHERE key=$1`, key).Scan(&val)
	if val == "" {
		return 0, 0, false
	}
	parts := strings.SplitN(val, ":", 2)
	h, err1 := strconv.Atoi(parts[0])
	m := 0
	if len(parts) == 2 {
		m, _ = strconv.Atoi(parts[1])
	}
	if err1 != nil {
		return 0, 0, false
	}
	return h, m, true
}

func (n *Notifier) checkBriefings(ctx context.Context, now time.Time) {
	today := kstToday(now)
	nowMin := now.Hour()*60 + now.Minute()

	// morning briefing
	if mh, mm, ok := n.settingHour(ctx, "briefing_morning"); ok {
		target := mh*60 + mm
		if nowMin >= target && nowMin <= target+1 {
			var sent *time.Time
			n.db.QueryRow(ctx, `SELECT MAX(sent_at) FROM briefing_log WHERE type='morning' AND sent_at > $1`, today).Scan(&sent)
			if sent == nil {
				if msg := n.buildMorningBriefing(ctx, now); msg != "" {
					if err := n.Send(ctx, msg); err == nil {
						n.db.Exec(ctx, `INSERT INTO briefing_log (type) VALUES ('morning')`)
					}
				}
			}
		}
	}

	// evening briefing
	if eh, em, ok := n.settingHour(ctx, "briefing_evening"); ok {
		target := eh*60 + em
		if nowMin >= target && nowMin <= target+1 {
			var sent *time.Time
			n.db.QueryRow(ctx, `SELECT MAX(sent_at) FROM briefing_log WHERE type='evening' AND sent_at > $1`, today).Scan(&sent)
			if sent == nil {
				if msg := n.buildEveningBriefing(ctx, now); msg != "" {
					if err := n.Send(ctx, msg); err == nil {
						n.db.Exec(ctx, `INSERT INTO briefing_log (type) VALUES ('evening')`)
					}
				}
			}
		}
	}
}

func (n *Notifier) buildMorningBriefing(ctx context.Context, now time.Time) string {
	today := kstToday(now)
	weekday := int(now.Weekday())

	// undone todos
	var todoCount int
	n.db.QueryRow(ctx, `SELECT COUNT(*) FROM items WHERE type='todo' AND done=false`).Scan(&todoCount)

	// due today or overdue
	var urgentCount int
	n.db.QueryRow(ctx, `SELECT COUNT(*) FROM items WHERE type='todo' AND done=false AND due_date <= CURRENT_DATE`).Scan(&urgentCount)

	// habits scheduled for today
	rows, _ := n.db.Query(ctx, `SELECT id, title, frequency, freq_days, last_checked_at FROM items WHERE type='habit'`)
	var habitsDue []string
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var title, freq, freqDays string
			var lastChecked *time.Time
			rows.Scan(&id, &title, &freq, &freqDays, &lastChecked)
			if !isScheduledDay(model.FreqType(freq), freqDays, now.Weekday()) {
				continue
			}
			checkedToday := lastChecked != nil && lastChecked.After(today)
			if !checkedToday {
				habitsDue = append(habitsDue, title)
			}
		}
	}
	_ = weekday

	dayNames := []string{"일요일", "월요일", "화요일", "수요일", "목요일", "금요일", "토요일"}
	msg := fmt.Sprintf("🌅 <b>좋은 아침이에요! (%s)</b>\n\n", dayNames[int(now.Weekday())])

	if todoCount > 0 {
		msg += fmt.Sprintf("📋 미완료 할 일: <b>%d개</b>", todoCount)
		if urgentCount > 0 {
			msg += fmt.Sprintf(" (⚠️ 오늘 마감 %d개)", urgentCount)
		}
		msg += "\n"
	} else {
		msg += "📋 오늘 할 일 없어요 🎉\n"
	}

	if len(habitsDue) > 0 {
		msg += fmt.Sprintf("\n🔄 오늘 체크할 습관 (%d개):\n", len(habitsDue))
		for _, h := range habitsDue {
			msg += fmt.Sprintf("  • %s\n", h)
		}
	} else {
		msg += "\n✅ 오늘 모든 습관 완료!\n"
	}

	return msg
}

func (n *Notifier) buildEveningBriefing(ctx context.Context, now time.Time) string {
	today := kstToday(now)

	// completed todos today
	doneRows, _ := n.db.Query(ctx, `SELECT title FROM items WHERE type='todo' AND done=true AND done_at > $1 ORDER BY done_at DESC LIMIT 10`, today)
	var doneTodos []string
	if doneRows != nil {
		defer doneRows.Close()
		for doneRows.Next() {
			var t string
			doneRows.Scan(&t)
			doneTodos = append(doneTodos, t)
		}
	}

	// still undone todos
	undoneRows, _ := n.db.Query(ctx, `SELECT title FROM items WHERE type='todo' AND done=false ORDER BY due_date ASC NULLS LAST LIMIT 10`)
	var undoneTodos []string
	if undoneRows != nil {
		defer undoneRows.Close()
		for undoneRows.Next() {
			var t string
			undoneRows.Scan(&t)
			undoneTodos = append(undoneTodos, t)
		}
	}

	// habits: checked vs not checked today
	hRows, _ := n.db.Query(ctx, `SELECT title, frequency, freq_days, last_checked_at FROM items WHERE type='habit'`)
	var checkedHabits, uncheckedHabits []string
	if hRows != nil {
		defer hRows.Close()
		for hRows.Next() {
			var title, freq, freqDays string
			var lastChecked *time.Time
			hRows.Scan(&title, &freq, &freqDays, &lastChecked)
			if !isScheduledDay(model.FreqType(freq), freqDays, now.Weekday()) {
				continue
			}
			if lastChecked != nil && lastChecked.After(today) {
				checkedHabits = append(checkedHabits, title)
			} else {
				uncheckedHabits = append(uncheckedHabits, title)
			}
		}
	}

	msg := "🌙 <b>오늘 하루 보고서</b>\n\n"

	if len(doneTodos) > 0 {
		msg += fmt.Sprintf("✅ <b>완료한 할 일 (%d개)</b>\n", len(doneTodos))
		for _, t := range doneTodos {
			msg += fmt.Sprintf("  • %s\n", t)
		}
		msg += "\n"
	}

	if len(undoneTodos) > 0 {
		msg += fmt.Sprintf("❌ <b>미완료 할 일 (%d개)</b>\n", len(undoneTodos))
		for _, t := range undoneTodos {
			msg += fmt.Sprintf("  • %s\n", t)
		}
		msg += "\n"
	} else if len(doneTodos) > 0 {
		msg += "🎉 모든 할 일 완료!\n\n"
	}

	total := len(checkedHabits) + len(uncheckedHabits)
	if total > 0 {
		msg += fmt.Sprintf("🔄 <b>습관 달성 %d/%d</b>\n", len(checkedHabits), total)
		for _, h := range checkedHabits {
			msg += fmt.Sprintf("  ✓ %s\n", h)
		}
		for _, h := range uncheckedHabits {
			msg += fmt.Sprintf("  ✗ %s\n", h)
		}
	}

	return msg
}
