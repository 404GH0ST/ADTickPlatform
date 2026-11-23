package database

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// poolMaxOpenConns reads the per-service connection-pool ceiling from
// DB_MAX_OPEN_CONNS, defaulting to 25. game-core raises this so its checker
// fan-out (capped at ~half the pool) can exceed the default; other services
// leave it unset and keep 25.
func poolMaxOpenConns() int {
	if v := strings.TrimSpace(os.Getenv("DB_MAX_OPEN_CONNS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 25
}

func OpenPostgres(ctx context.Context, dsn string) (*sql.DB, error) {
	// sql.Open cannot fail for the registered pgx driver: its OpenConnector
	// defers DSN parsing to the first connection, which PingContext surfaces.
	db, _ := sql.Open("pgx", dsn)

	maxOpen := poolMaxOpenConns()
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxOpen)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
