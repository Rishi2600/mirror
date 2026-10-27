// Package db opens the Postgres connection and runs the migrations.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"mirror/server/migrations"
)

const pingTimeout = 5 * time.Second

// Open connects to Postgres and checks that it answers.
func Open(url string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{
		Logger:               logger.Discard,
		TranslateError:       true, // turns unique violations into gorm.ErrDuplicatedKey
		DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return db, nil
}

// Close closes the connection pool.
func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Migrate applies any migrations that have not run yet and returns how many
// ran. A Postgres lock stops two processes from migrating at the same time.
func Migrate(ctx context.Context, db *gorm.DB) (int, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return 0, err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return 0, err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS,
		goose.WithSessionLocker(locker))
	if err != nil {
		return 0, err
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("running migrations: %w", err)
	}
	return len(results), nil
}
