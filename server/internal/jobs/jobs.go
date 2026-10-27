// Package jobs runs background work with River, a job queue stored in our
// Postgres. Jobs survive a server restart and are retried on errors.
package jobs

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"gorm.io/gorm"

	"mirror/server/internal/profiles"
	"mirror/server/internal/storage"
)

// ParseCVArgs asks for one uploaded CV file to be read into the profile.
type ParseCVArgs struct {
	UserID string `json:"user_id"`
	FileID string `json:"file_id"`
}

func (ParseCVArgs) Kind() string { return "parse_cv" }

// Three tries: Groq's free tier can rate-limit for a moment.
func (ParseCVArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 3} }

type parseCVWorker struct {
	river.WorkerDefaults[ParseCVArgs]
	db     *gorm.DB
	files  *storage.Local
	parser profiles.CVParser
}

func (w *parseCVWorker) Work(ctx context.Context, job *river.Job[ParseCVArgs]) error {
	err := profiles.ParseCV(ctx, w.db, w.files, w.parser, job.Args.UserID, job.Args.FileID)
	if err != nil && job.Attempt >= job.MaxAttempts {
		profiles.MarkFailed(ctx, w.db, job.Args.UserID, job.Args.FileID,
			"we could not read your CV right now; please upload it again")
	}
	return err
}

// Queue runs the workers and accepts new jobs.
type Queue struct {
	client *river.Client[pgx.Tx]
	pool   *pgxpool.Pool
}

// Start creates River's tables if needed and starts the workers. River uses
// its own pgx connection pool; GORM keeps its own.
func Start(ctx context.Context, databaseURL string, db *gorm.DB, files *storage.Local, parser profiles.CVParser, logger *slog.Logger) (*Queue, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	driver := riverpgxv5.New(pool)

	migrator, err := rivermigrate.New(driver, nil)
	if err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		pool.Close()
		return nil, err
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, &parseCVWorker{db: db, files: files, parser: parser})
	client, err := river.NewClient(driver, &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}},
		Workers: workers,
		Logger:  logger,
	})
	if err != nil {
		pool.Close()
		return nil, err
	}
	if err := client.Start(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Queue{client: client, pool: pool}, nil
}

// ParseCV queues the parse job for an uploaded CV.
func (q *Queue) ParseCV(ctx context.Context, userID, fileID string) error {
	_, err := q.client.Insert(ctx, ParseCVArgs{UserID: userID, FileID: fileID}, nil)
	return err
}

// Stop waits for running jobs to finish, then closes the pool.
func (q *Queue) Stop(ctx context.Context) error {
	err := q.client.Stop(ctx)
	q.pool.Close()
	return err
}
