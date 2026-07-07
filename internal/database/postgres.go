package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	config.MaxConns = 10
	config.MinConns = 1
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return pool, nil
}

// migrate creates missing tables and upgrades older tables safely.
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	fmt.Println("🔄 Running database migrations...")
	statements := []string{
		`SELECT 1`, // Test connection first
		`CREATE TABLE IF NOT EXISTS users (
			id            SERIAL PRIMARY KEY,
			name          VARCHAR(100) NOT NULL,
			username      VARCHAR(50),
			email         VARCHAR(100) UNIQUE NOT NULL,
			phone         VARCHAR(15) UNIQUE NOT NULL,
			age           INT,
			profile_photo_path TEXT,
			password_hash TEXT NOT NULL,
			created_at    TIMESTAMP DEFAULT NOW(),
			updated_at    TIMESTAMP DEFAULT NOW()
		)`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR(100)`,
		`UPDATE users SET name = 'User' || id::text WHERE name IS NULL`,
		`ALTER TABLE users ALTER COLUMN name SET NOT NULL`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS age INT`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS profile_photo_path TEXT`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS username VARCHAR(50)`,
		`UPDATE users
			SET username = 'user_' || id::text
			WHERE username IS NULL OR username = ''`,
		`ALTER TABLE users ALTER COLUMN username SET NOT NULL`,
		`UPDATE users SET created_at = NOW() WHERE created_at IS NULL`,
		`UPDATE users SET updated_at = NOW() WHERE updated_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone ON users(phone)`,
		`CREATE TABLE IF NOT EXISTS otp_codes (
			id         SERIAL PRIMARY KEY,
			email      VARCHAR(100) NOT NULL,
			code       CHAR(6) NOT NULL,
			purpose    VARCHAR(30) NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			used       BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_otp_email_purpose ON otp_codes(email, purpose)`,
	}

	for i, stmt := range statements {
		fmt.Printf("  Migration %d: %s\n", i+1, preview(stmt))
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("exec migration %q: %w", preview(stmt), err)
		}
	}

	fmt.Println("✅ Database migrations completed successfully")
	return nil
}

func preview(stmt string) string {
	if len(stmt) <= 60 {
		return stmt
	}
	return stmt[:60]
}
