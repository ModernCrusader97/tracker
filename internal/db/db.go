package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func New(url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, err
	}
	if err := migrate(pool); err != nil {
		return nil, err
	}
	return pool, nil
}

func migrate(pool *pgxpool.Pool) error {
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS items (
			id               BIGSERIAL PRIMARY KEY,
			type             VARCHAR(10) NOT NULL,          -- 'todo' | 'habit'
			title            VARCHAR(300) NOT NULL,
			note             TEXT NOT NULL DEFAULT '',
			done             BOOLEAN NOT NULL DEFAULT FALSE,
			done_at          TIMESTAMPTZ,
			due_date         DATE,
			remind_every_min INT NOT NULL DEFAULT 0,
			frequency        VARCHAR(10) NOT NULL DEFAULT 'daily',
			reminder_hour    INT NOT NULL DEFAULT 21,
			streak           INT NOT NULL DEFAULT 0,
			last_checked_at  TIMESTAMPTZ,
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE items ADD COLUMN IF NOT EXISTS due_date DATE;

		CREATE TABLE IF NOT EXISTS reminder_log (
			id         BIGSERIAL PRIMARY KEY,
			item_id    BIGINT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
			sent_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS habit_checks (
			id         BIGSERIAL PRIMARY KEY,
			item_id    BIGINT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
			checked_on DATE NOT NULL DEFAULT CURRENT_DATE,
			UNIQUE(item_id, checked_on)
		);

		CREATE TABLE IF NOT EXISTS settings (
			key   VARCHAR(100) PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		);
		ALTER TABLE items ADD COLUMN IF NOT EXISTS snoozed_until TIMESTAMPTZ;
		ALTER TABLE items ALTER COLUMN frequency TYPE VARCHAR(20);
		ALTER TABLE items ADD COLUMN IF NOT EXISTS remind_before_min INT NOT NULL DEFAULT 0;
		ALTER TABLE items ADD COLUMN IF NOT EXISTS category TEXT NOT NULL DEFAULT '';
		ALTER TABLE items ADD COLUMN IF NOT EXISTS weekly_goal INT NOT NULL DEFAULT 0;
		CREATE TABLE IF NOT EXISTS briefing_log (
			id         BIGSERIAL PRIMARY KEY,
			type       VARCHAR(10) NOT NULL,
			sent_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE items ADD COLUMN IF NOT EXISTS reminder_minute INT NOT NULL DEFAULT 0;
		ALTER TABLE items ADD COLUMN IF NOT EXISTS freq_days TEXT NOT NULL DEFAULT '';
		ALTER TABLE reminder_log ADD COLUMN IF NOT EXISTS remind_type VARCHAR(10) NOT NULL DEFAULT 'main';
		ALTER TABLE items ADD COLUMN IF NOT EXISTS exclude_holidays BOOLEAN NOT NULL DEFAULT FALSE;
		ALTER TABLE items ADD COLUMN IF NOT EXISTS tags TEXT NOT NULL DEFAULT '';
		ALTER TABLE items ADD COLUMN IF NOT EXISTS icon TEXT NOT NULL DEFAULT '';
		CREATE TABLE IF NOT EXISTS notification_queue (
			id        BIGSERIAL PRIMARY KEY,
			text      TEXT NOT NULL,
			item_id   BIGINT,
			item_type VARCHAR(10),
			sent_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE IF NOT EXISTS users (
			id          BIGSERIAL PRIMARY KEY,
			telegram_id BIGINT UNIQUE NOT NULL,
			username    TEXT NOT NULL DEFAULT '',
			first_name  TEXT NOT NULL DEFAULT '',
			photo_url   TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		ALTER TABLE items ADD COLUMN IF NOT EXISTS user_id BIGINT REFERENCES users(id) ON DELETE CASCADE;
	`)
	return err
}
