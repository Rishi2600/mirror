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

	"mirror/server/internal/embed"
	"mirror/server/internal/matching"
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
	if err != nil {
		return err
	}
	// A new CV changes a finished card, so refresh its vector. (Tests run
	// Work without a River client; there is nothing to queue then.)
	if client, cerr := river.ClientFromContextSafely[pgx.Tx](ctx); cerr == nil {
		_, err = client.Insert(ctx, EmbedArgs{OwnerType: matching.OwnerCandidate, OwnerID: job.Args.UserID}, nil)
	}
	return err
}

// EmbedArgs asks for one card's meaning vector to be made or refreshed.
type EmbedArgs struct {
	OwnerType string `json:"owner_type"`
	OwnerID   string `json:"owner_id"`
}

func (EmbedArgs) Kind() string { return "embed" }

// One waiting job per card is enough: it embeds whatever the card says
// when it runs.
func (EmbedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 5, UniqueOpts: river.UniqueOpts{ByArgs: true}}
}

type embedWorker struct {
	river.WorkerDefaults[EmbedArgs]
	db       *gorm.DB
	embedder embed.Embedder
}

func (w *embedWorker) Work(ctx context.Context, job *river.Job[EmbedArgs]) error {
	return matching.EmbedOwner(ctx, w.db, w.embedder, job.Args.OwnerType, job.Args.OwnerID)
}

// Queue runs the workers and accepts new jobs.
type Queue struct {
	client *river.Client[pgx.Tx]
	pool   *pgxpool.Pool
}

// Start creates River's tables if needed and starts the workers. River uses
// its own pgx connection pool; GORM keeps its own.
func Start(ctx context.Context, databaseURL string, db *gorm.DB, files *storage.Local, parser profiles.CVParser, embedder embed.Embedder, logger *slog.Logger) (*Queue, error) {
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
	river.AddWorker(workers, &embedWorker{db: db, embedder: embedder})
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

// EmbedCandidate queues a refresh of a candidate card's vector.
func (q *Queue) EmbedCandidate(ctx context.Context, userID string) error {
	_, err := q.client.Insert(ctx, EmbedArgs{OwnerType: matching.OwnerCandidate, OwnerID: userID}, nil)
	return err
}

// EmbedHiringCard queues a refresh of a hiring card's vector.
func (q *Queue) EmbedHiringCard(ctx context.Context, cardID string) error {
	_, err := q.client.Insert(ctx, EmbedArgs{OwnerType: matching.OwnerHiringCard, OwnerID: cardID}, nil)
	return err
}

// Backfill queues a vector for every live card that has none from the
// current model: cards from before this chunk, and all cards after a model
// change. It returns how many it queued.
func (q *Queue) Backfill(ctx context.Context, db *gorm.DB) (int, error) {
	owners, err := matching.MissingEmbeddings(ctx, db)
	if err != nil {
		return 0, err
	}
	for _, o := range owners {
		if _, err := q.client.Insert(ctx, EmbedArgs{OwnerType: o.Type, OwnerID: o.ID}, nil); err != nil {
			return 0, err
		}
	}
	return len(owners), nil
}

// Stop waits for running jobs to finish, then closes the pool.
func (q *Queue) Stop(ctx context.Context) error {
	err := q.client.Stop(ctx)
	q.pool.Close()
	return err
}
