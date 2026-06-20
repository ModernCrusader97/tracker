package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"tracker/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ItemService struct {
	db *pgxpool.Pool
}

func New(db *pgxpool.Pool) *ItemService { return &ItemService{db: db} }

func (s *ItemService) DB() *pgxpool.Pool { return s.db }

var kst = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		return time.FixedZone("KST", 9*60*60)
	}
	return loc
}()

func kstToday(t time.Time) time.Time {
	t = t.In(kst)
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, kst)
}

func daysLeft(dueDate *string) *int {
	if dueDate == nil || *dueDate == "" {
		return nil
	}
	due, err := time.Parse("2006-01-02", *dueDate)
	if err != nil {
		return nil
	}
	today := kstToday(time.Now())
	d := int(due.Sub(today).Hours() / 24)
	return &d
}

const itemCols = `id, type, title, note, done, done_at,
	to_char(due_date, 'YYYY-MM-DD'),
	remind_every_min,
	frequency, freq_days, reminder_hour, reminder_minute, remind_before_min,
	weekly_goal, streak, last_checked_at, created_at, exclude_holidays,
	(SELECT COUNT(*) FROM habit_checks WHERE item_id=items.id AND checked_on >= date_trunc('week', CURRENT_DATE)) AS week_count,
	tags, icon`

func scanItem(row interface {
	Scan(...any) error
}, it *model.Item) error {
	return row.Scan(
		&it.ID, &it.Type, &it.Title, &it.Note, &it.Done, &it.DoneAt,
		&it.DueDate,
		&it.RemindEveryMin,
		&it.Frequency, &it.FreqDays, &it.ReminderHour, &it.ReminderMinute, &it.RemindBeforeMin,
		&it.WeeklyGoal, &it.Streak, &it.LastCheckedAt, &it.CreatedAt, &it.ExcludeHolidays,
		&it.WeekCount, &it.Tags, &it.Icon,
	)
}

func (s *ItemService) List(ctx context.Context, userID int64) ([]*model.Item, error) {
	rows, err := s.db.Query(ctx, `SELECT `+itemCols+` FROM items WHERE user_id=$1 ORDER BY done ASC, due_date ASC NULLS LAST, created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	today := kstToday(time.Now())
	var items []*model.Item
	for rows.Next() {
		it := &model.Item{}
		if err := scanItem(rows, it); err != nil {
			continue
		}
		if it.LastCheckedAt != nil {
			it.CheckedToday = it.LastCheckedAt.After(today)
		}
		it.DaysLeft = daysLeft(it.DueDate)
		items = append(items, it)
	}
	if items == nil {
		items = []*model.Item{}
	}

	// Batch-load last_notified_at for items with reminders (avoids N+1 queries)
	var remindIDs []int64
	for _, it := range items {
		if it.RemindEveryMin > 0 {
			remindIDs = append(remindIDs, it.ID)
		}
	}
	if len(remindIDs) > 0 {
		notifRows, err := s.db.Query(ctx,
			`SELECT item_id, MAX(sent_at) FROM reminder_log WHERE item_id = ANY($1) GROUP BY item_id`,
			remindIDs)
		if err == nil {
			defer notifRows.Close()
			lastNotifMap := map[int64]*time.Time{}
			for notifRows.Next() {
				var id int64
				var t time.Time
				if notifRows.Scan(&id, &t) == nil {
					tCopy := t
					lastNotifMap[id] = &tCopy
				}
			}
			for _, it := range items {
				it.LastNotifiedAt = lastNotifMap[it.ID]
			}
		}
	}

	return items, nil
}

func (s *ItemService) Create(ctx context.Context, req *model.CreateItemRequest, userID int64) (*model.Item, error) {
	if req.Type == model.TypeHabit && req.Frequency == "" {
		req.Frequency = model.FreqDaily
	}
	var dueDate *string
	if req.DueDate != "" {
		dueDate = &req.DueDate
	}
	if req.Tags == "" {
		req.Tags = AutoTag(req.Title + " " + req.Note)
	}
	if req.Icon == "" {
		req.Icon = AutoIcon(req.Tags)
	}
	it := &model.Item{}
	err := scanItem(s.db.QueryRow(ctx, `
		INSERT INTO items (type, title, note, due_date, remind_every_min,
		                   frequency, freq_days, reminder_hour, reminder_minute, remind_before_min, weekly_goal, exclude_holidays, tags, icon, user_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING `+itemCols,
		req.Type, req.Title, req.Note, dueDate, req.RemindEveryMin,
		req.Frequency, req.FreqDays, req.ReminderHour, req.ReminderMinute, req.RemindBeforeMin, req.WeeklyGoal, req.ExcludeHolidays,
		req.Tags, req.Icon, userID,
	), it)
	it.DaysLeft = daysLeft(it.DueDate)
	return it, err
}

func (s *ItemService) Update(ctx context.Context, id, userID int64, req *model.UpdateItemRequest) (*model.Item, error) {
	it := &model.Item{}
	var err error
	if err = scanItem(s.db.QueryRow(ctx, `SELECT `+itemCols+` FROM items WHERE id=$1 AND ($2=0 OR user_id=$2)`, id, userID), it); err != nil {
		return nil, fmt.Errorf("not found")
	}

	if req.Title != nil { it.Title = *req.Title }
	if req.Note != nil { it.Note = *req.Note }
	if req.RemindEveryMin != nil { it.RemindEveryMin = *req.RemindEveryMin }
	if req.Frequency != nil { it.Frequency = model.FreqType(*req.Frequency) }
	if req.FreqDays != nil { it.FreqDays = *req.FreqDays }
	if req.ReminderHour != nil { it.ReminderHour = *req.ReminderHour }
	if req.ReminderMinute != nil { it.ReminderMinute = *req.ReminderMinute }
	if req.RemindBeforeMin != nil { it.RemindBeforeMin = *req.RemindBeforeMin }
	if req.WeeklyGoal != nil { it.WeeklyGoal = *req.WeeklyGoal }
	if req.ExcludeHolidays != nil { it.ExcludeHolidays = *req.ExcludeHolidays }
	if req.Tags != nil { it.Tags = *req.Tags }
	if req.Icon != nil { it.Icon = *req.Icon }
	if req.DueDate != nil {
		if *req.DueDate == "" {
			it.DueDate = nil
		} else {
			it.DueDate = req.DueDate
		}
	}
	if req.Done != nil {
		it.Done = *req.Done
		if it.Done {
			now := time.Now()
			it.DoneAt = &now
		} else {
			it.DoneAt = nil
		}
	}

	_, err = s.db.Exec(ctx, `
		UPDATE items SET title=$2, note=$3, done=$4, done_at=$5, due_date=$6,
		  remind_every_min=$7, frequency=$8, freq_days=$9, reminder_hour=$10, reminder_minute=$11,
		  remind_before_min=$12, weekly_goal=$13, exclude_holidays=$14, tags=$15, icon=$16
		WHERE id=$1`,
		it.ID, it.Title, it.Note, it.Done, it.DoneAt, it.DueDate,
		it.RemindEveryMin, it.Frequency, it.FreqDays, it.ReminderHour, it.ReminderMinute,
		it.RemindBeforeMin, it.WeeklyGoal, it.ExcludeHolidays, it.Tags, it.Icon)
	it.DaysLeft = daysLeft(it.DueDate)
	if it.LastCheckedAt != nil {
		it.CheckedToday = it.LastCheckedAt.After(kstToday(time.Now()))
	}
	return it, err
}

func (s *ItemService) CheckHabit(ctx context.Context, id, userID int64) (*model.Item, error) {
	it := &model.Item{}
	err := s.db.QueryRow(ctx, `SELECT id, type, frequency, freq_days, streak, last_checked_at FROM items WHERE id=$1 AND type='habit' AND ($2=0 OR user_id=$2)`, id, userID).
		Scan(&it.ID, &it.Type, &it.Frequency, &it.FreqDays, &it.Streak, &it.LastCheckedAt)
	if err != nil {
		return nil, fmt.Errorf("not found")
	}

	now := time.Now()
	today := kstToday(now)
	if it.LastCheckedAt != nil && it.LastCheckedAt.After(today) {
		return nil, fmt.Errorf("오늘 이미 체크했습니다")
	}

	newStreak := 1
	if it.LastCheckedAt != nil {
		prev := prevScheduledDay(it.Frequency, it.FreqDays, today)
		if it.LastCheckedAt.After(prev) {
			newStreak = it.Streak + 1
		}
	}

	err = scanItem(s.db.QueryRow(ctx, `
		UPDATE items SET last_checked_at=$2, streak=$3 WHERE id=$1
		RETURNING `+itemCols,
		id, now, newStreak,
	), it)
	if err != nil {
		return nil, err
	}
	s.db.Exec(ctx, `INSERT INTO habit_checks (item_id, checked_on) VALUES ($1, CURRENT_DATE) ON CONFLICT DO NOTHING`, id)
	it.CheckedToday = true
	return it, nil
}

func prevScheduledDay(freq model.FreqType, freqDays string, today time.Time) time.Time {
	for i := 1; i <= 14; i++ {
		d := today.AddDate(0, 0, -i)
		if isScheduledDay(freq, freqDays, d.Weekday()) {
			return d
		}
	}
	return today.AddDate(0, 0, -1)
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

func (s *ItemService) HeatmapData(ctx context.Context, id int64) ([]string, error) {
	rows, err := s.db.Query(ctx, `
		SELECT to_char(checked_on, 'YYYY-MM-DD') FROM habit_checks
		WHERE item_id=$1 AND checked_on >= CURRENT_DATE - INTERVAL '83 days'
		ORDER BY checked_on ASC`, id)
	if err != nil {
		return []string{}, nil
	}
	defer rows.Close()
	var dates []string
	for rows.Next() {
		var d string
		rows.Scan(&d)
		dates = append(dates, d)
	}
	if dates == nil {
		dates = []string{}
	}
	return dates, nil
}

func (s *ItemService) Delete(ctx context.Context, id, userID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM items WHERE id=$1 AND user_id=$2`, id, userID)
	return err
}

// FindOrCreateUser upserts a user by telegram_id and claims any unclaimed items
// for the first user created.
func (s *ItemService) FindOrCreateUser(ctx context.Context, telegramID int64, username, firstName, photoURL string) (*model.User, error) {
	u := &model.User{}
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (telegram_id, username, first_name, photo_url)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (telegram_id) DO UPDATE
		  SET username=$2, first_name=$3, photo_url=$4
		RETURNING id, telegram_id, username, first_name, photo_url, created_at`,
		telegramID, username, firstName, photoURL,
	).Scan(&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.PhotoURL, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	// Claim any items not yet assigned to a user.
	s.db.Exec(ctx, `UPDATE items SET user_id=$1 WHERE user_id IS NULL`, u.ID)
	return u, nil
}

func (s *ItemService) Stats(ctx context.Context, userID int64) (*model.StatsResponse, error) {
	resp := &model.StatsResponse{}

	// habit stats: last 30 days
	habitRows, err := s.db.Query(ctx, `SELECT id, title, icon, streak, frequency, freq_days FROM items WHERE type='habit' AND user_id=$1`, userID)
	if err != nil {
		return resp, nil
	}
	defer habitRows.Close()
	type habitMeta struct {
		id, streak       int64
		title, icon, freq, freqDays string
	}
	var habits []habitMeta
	for habitRows.Next() {
		var h habitMeta
		habitRows.Scan(&h.id, &h.title, &h.icon, &h.streak, &h.freq, &h.freqDays)
		habits = append(habits, h)
	}
	habitRows.Close()

	today := kstToday(time.Now())
	for _, h := range habits {
		hs := model.HabitStat{ID: h.id, Title: h.title, Icon: h.icon, Streak: int(h.streak)}
		scheduledCount := 0
		checkedCount := 0
		for i := 29; i >= 0; i-- {
			day := today.AddDate(0, 0, -i)
			if !isScheduledDay(model.FreqType(h.freq), h.freqDays, day.Weekday()) {
				continue
			}
			scheduledCount++
			dateStr := day.Format("2006-01-02")
			var cnt int
			s.db.QueryRow(ctx, `SELECT COUNT(*) FROM habit_checks WHERE item_id=$1 AND checked_on=$2`, h.id, dateStr).Scan(&cnt)
			checked := cnt > 0
			if checked { checkedCount++ }
			hs.DailyChecks = append(hs.DailyChecks, model.DayCheck{Date: dateStr, Checked: checked})
		}
		if scheduledCount > 0 {
			hs.Rate30d = float64(checkedCount) / float64(scheduledCount)
		}
		resp.Habits = append(resp.Habits, hs)
	}

	// todo completion by day (last 30 days)
	todoRows, err := s.db.Query(ctx, `
		SELECT to_char(done_at::date, 'YYYY-MM-DD'), COUNT(*)
		FROM items WHERE type='todo' AND done=true AND done_at >= NOW() - INTERVAL '30 days' AND user_id=$1
		GROUP BY done_at::date ORDER BY done_at::date ASC`, userID)
	if err == nil {
		defer todoRows.Close()
		for todoRows.Next() {
			var ts model.TodoStat
			todoRows.Scan(&ts.Date, &ts.Count)
			resp.TodosCompleted = append(resp.TodosCompleted, ts)
		}
	}
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM items WHERE type='todo' AND done=true AND user_id=$1`, userID).Scan(&resp.TotalCompleted)
	s.db.QueryRow(ctx, `SELECT COUNT(*) FROM items WHERE type='todo' AND done=false AND user_id=$1`, userID).Scan(&resp.TotalPending)

	return resp, nil
}

var tagKeywords = map[string][]string{
	"운동":  {"운동", "헬스", "달리기", "러닝", "걷기", "수영", "요가", "스트레칭", "자전거", "등산", "gym", "exercise", "workout", "run", "walk"},
	"건강":  {"건강", "약", "물마시기", "비타민", "병원", "의사", "수면", "잠자기", "체중", "다이어트", "health", "medicine", "sleep", "water"},
	"공부":  {"공부", "학습", "독서", "책", "강의", "수업", "시험", "코딩", "프로그래밍", "study", "learn", "read", "book", "course", "coding"},
	"업무":  {"업무", "일", "회의", "미팅", "보고서", "이메일", "프로젝트", "마감", "work", "meeting", "report", "email", "project", "deadline"},
	"식사":  {"식사", "밥", "점심", "저녁", "요리", "음식", "먹기", "식단", "아침밥", "아침식사", "meal", "lunch", "dinner", "breakfast", "cook", "eat", "food"},
	"청소":  {"청소", "정리", "빨래", "설거지", "쓰레기", "청결", "clean", "laundry", "tidy", "wash"},
	"쇼핑":  {"쇼핑", "구매", "구입", "마트", "장보기", "주문", "배송", "shop", "buy", "order", "market"},
	"사교":  {"친구", "가족", "모임", "약속", "연락", "전화", "friend", "family", "meet", "call", "social"},
	"취미":  {"취미", "게임", "영화", "드라마", "음악", "그림", "hobby", "game", "movie", "music", "art", "draw"},
	"재정":  {"돈", "저축", "투자", "가계부", "보험", "세금", "money", "save", "invest", "budget", "finance", "tax"},
}

var tagIcons = map[string]string{
	"운동": "🏃", "건강": "💊", "공부": "📚", "업무": "💼",
	"식사": "🍽️", "청소": "🧹", "쇼핑": "🛒", "사교": "👥",
	"취미": "🎮", "재정": "💰",
}

func AutoTag(text string) string {
	lower := strings.ToLower(text)
	var matched []string
	for tag, keywords := range tagKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				matched = append(matched, tag)
				break
			}
		}
	}
	return strings.Join(matched, ",")
}

func AutoIcon(tags string) string {
	priority := []string{"운동", "건강", "공부", "업무", "사교", "식사", "청소", "쇼핑", "취미", "재정"}
	tagSet := map[string]bool{}
	for _, t := range strings.Split(tags, ",") {
		tagSet[strings.TrimSpace(t)] = true
	}
	for _, p := range priority {
		if tagSet[p] {
			return tagIcons[p]
		}
	}
	return ""
}
