package database

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenPostgres(ctx context.Context, dsn string) (*sql.DB, error) {
	// sql.Open cannot fail for the registered pgx driver: its OpenConnector
	// defers DSN parsing to the first connection, which PingContext surfaces.
	db, _ := sql.Open("pgx", dsn)

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
