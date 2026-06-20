package model

import "time"

type User struct {
	ID         int64     `json:"id"`
	TelegramID int64     `json:"telegram_id"`
	Username   string    `json:"username"`
	FirstName  string    `json:"first_name"`
	PhotoURL   string    `json:"photo_url"`
	CreatedAt  time.Time `json:"created_at"`
}

type TodoType string

const (
	TypeTodo  TodoType = "todo"
	TypeHabit TodoType = "habit"
)

type FreqType string

const (
	FreqDaily      FreqType = "daily"
	FreqWeekly     FreqType = "weekly"
	FreqDaysOfWeek FreqType = "days_of_week"
)

type Item struct {
	ID              int64      `json:"id"`
	Type            TodoType   `json:"type"`
	Title           string     `json:"title"`
	Note            string     `json:"note,omitempty"`
	Done            bool       `json:"done"`
	DoneAt          *time.Time `json:"done_at,omitempty"`
	DueDate         *string    `json:"due_date,omitempty"` // "YYYY-MM-DD"
	DaysLeft        *int       `json:"days_left,omitempty"`
	// todo-specific
	RemindEveryMin  int        `json:"remind_every_min,omitempty"`
	// habit-specific
	Frequency       FreqType   `json:"frequency,omitempty"`
	FreqDays        string     `json:"freq_days,omitempty"`
	ReminderHour    int        `json:"reminder_hour"`
	ReminderMinute  int        `json:"reminder_minute,omitempty"`
	RemindBeforeMin int        `json:"remind_before_min,omitempty"`
	WeeklyGoal      int        `json:"weekly_goal,omitempty"`
	WeekCount       int        `json:"week_count,omitempty"`
	Streak          int        `json:"streak"`
	LastCheckedAt   *time.Time `json:"last_checked_at,omitempty"`
	CheckedToday    bool       `json:"checked_today"`
	ExcludeHolidays bool       `json:"exclude_holidays,omitempty"`
	Tags            string     `json:"tags,omitempty"`
	Icon            string     `json:"icon,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	LastNotifiedAt  *time.Time `json:"last_notified_at,omitempty"`
}

type CreateItemRequest struct {
	Type            TodoType `json:"type" binding:"required"`
	Title           string   `json:"title" binding:"required"`
	Note            string   `json:"note"`
	DueDate         string   `json:"due_date"`
	RemindEveryMin  int      `json:"remind_every_min"`
	Frequency       FreqType `json:"frequency"`
	FreqDays        string   `json:"freq_days"`
	ReminderHour    int      `json:"reminder_hour"`
	ReminderMinute  int      `json:"reminder_minute"`
	RemindBeforeMin int      `json:"remind_before_min"`
	WeeklyGoal      int      `json:"weekly_goal"`
	ExcludeHolidays bool     `json:"exclude_holidays"`
	Tags            string   `json:"tags"`
	Icon            string   `json:"icon"`
}

type UpdateItemRequest struct {
	Title           *string  `json:"title"`
	Note            *string  `json:"note"`
	Done            *bool    `json:"done"`
	DueDate         *string  `json:"due_date"`
	RemindEveryMin  *int     `json:"remind_every_min"`
	Frequency       *string  `json:"frequency"`
	FreqDays        *string  `json:"freq_days"`
	ReminderHour    *int     `json:"reminder_hour"`
	ReminderMinute  *int     `json:"reminder_minute"`
	RemindBeforeMin *int     `json:"remind_before_min"`
	WeeklyGoal      *int     `json:"weekly_goal"`
	ExcludeHolidays *bool    `json:"exclude_holidays"`
	Tags            *string  `json:"tags"`
	Icon            *string  `json:"icon"`
}

type HabitStat struct {
	ID             int64   `json:"id"`
	Title          string  `json:"title"`
	Icon           string  `json:"icon"`
	Streak         int     `json:"streak"`
	Rate30d        float64 `json:"rate_30d"`
	DailyChecks    []DayCheck `json:"daily_checks"`
}

type DayCheck struct {
	Date    string `json:"date"`
	Checked bool   `json:"checked"`
}

type TodoStat struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type StatsResponse struct {
	Habits         []HabitStat `json:"habits"`
	TodosCompleted []TodoStat  `json:"todos_completed"`
	TotalCompleted int         `json:"total_completed"`
	TotalPending   int         `json:"total_pending"`
}
