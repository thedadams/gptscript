package threads

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/datatypes"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type Store struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

func NewStore(dsn string) (*Store, error) {
	var (
		gdb   gorm.Dialector
		conns = 1
	)
	switch {
	case strings.HasPrefix(dsn, "sqlite://"):
		gdb = sqlite.Open(strings.TrimPrefix(dsn, "sqlite://"))
	case strings.HasPrefix(dsn, "postgres://"):
		conns = 5
		gdb = postgres.Open(dsn)
	case strings.HasPrefix(dsn, "mysql://"):
		conns = 5
		gdb = mysql.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", dsn)
	}

	db, err := gorm.Open(gdb, &gorm.Config{
		SkipDefaultTransaction: true,
		Logger: logger.New(log.Default(), logger.Config{
			SlowThreshold: 200 * time.Millisecond,
			Colorful:      true,
			LogLevel:      logger.Silent,
		}),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetConnMaxLifetime(3 * time.Minute)
	sqlDB.SetMaxIdleConns(conns)
	sqlDB.SetMaxOpenConns(conns)

	return &Store{db: db, sqlDB: sqlDB}, nil
}

func (s *Store) Close() error {
	return s.sqlDB.Close()
}

func (s *Store) CreateThread(ctx context.Context) (*Thread, error) {
	if s == nil {
		return nil, nil
	}

	thread := new(Thread)
	return thread, s.db.WithContext(ctx).Clauses(clause.Returning{}).Create(thread).Error
}

func (s *Store) NameThread(ctx context.Context, id uint64, name string) (*Thread, error) {
	if s == nil {
		return nil, nil
	}

	thread := new(Thread)
	return thread, s.db.WithContext(ctx).Model(thread).Clauses(clause.Returning{}).Where("id = ?", id).Update("name", name).Error
}

func (s *Store) DeleteThread(ctx context.Context, id uint64) error {
	if s == nil {
		return nil
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var runs []Run
		if err := tx.Where("thread_id = ?", id).Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).Delete(&runs).Error; err != nil {
			return err
		}

		// TODO(thedadams): test to ensure that this works.
		if err := tx.Where("run_id IN ?", runs).Delete(new(Event)).Error; err != nil {
			return err
		}

		return tx.Where("id = ?", id).Delete(new(Thread)).Error
	})
}

func (s *Store) ListThreads(ctx context.Context) ([]Thread, error) {
	if s == nil {
		return nil, nil
	}

	var threads []Thread
	return threads, s.db.WithContext(ctx).Find(&threads).Error
}

func (s *Store) GetThread(ctx context.Context, id uint64) (*Thread, error) {
	if s == nil {
		return nil, nil
	}

	thread := new(Thread)
	return thread, s.db.WithContext(ctx).Where("id = ?", id).First(thread).Error
}

func (s *Store) CreateRun(ctx context.Context, input string, previousRunID uint64, run *RunInfo) (*Run, error) {
	if s == nil {
		return nil, nil
	}

	r := &Run{
		PreviousRunID: previousRunID,
		StartedAt:     time.Now(),
		Input:         input,
		Run:           datatypes.NewJSONType(run),
	}

	return r, s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if previousRunID != 0 {
			previousRun := new(Run)
			if err := tx.Where("id = ?", previousRunID).First(previousRun).Error; err != nil {
				return err
			}

			r.ThreadID = previousRun.ThreadID
		}

		return tx.Clauses(clause.Returning{}).Create(r).Error
	})
}

func (s *Store) FinishRun(ctx context.Context, threadID, id uint64, r *RunInfo) (*Run, error) {
	if s == nil {
		return nil, nil
	}

	r.ThreadID = threadID
	run := &Run{
		ID:             id,
		ThreadID:       threadID,
		FinishedAt:     time.Now(),
		Output:         r.Output,
		ChatStateAfter: r.ChatState,
		Run:            datatypes.NewJSONType(r),
		Calls:          datatypes.NewJSONType(r.Calls),
	}
	return run, s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if threadID == 0 {
			// If the run wasn't put into a thread, then delete the run and its events because they aren't needed.
			if err := tx.Clauses(clause.Returning{}).Delete(run).Error; err != nil {
				return err
			}
			return tx.Where("run_id = ?", id).Delete(new(Event)).Error
		}

		return tx.Clauses(clause.Returning{}).Updates(run).Error
	})
}

func (s *Store) ListRuns(ctx context.Context, threadID uint64) ([]Run, error) {
	if s == nil {
		return nil, nil
	}

	var runs []Run
	return runs, s.db.WithContext(ctx).Where("thread_id = ?", threadID).Order("id ASC").Find(&runs).Error
}

func (s *Store) GetRun(ctx context.Context, threadID, id uint64) (*Run, error) {
	if s == nil {
		return nil, nil
	}

	run := new(Run)
	return run, s.db.WithContext(ctx).Where("thread_id = ? AND id = ?", threadID, id).First(run).Error
}

func (s *Store) CreateEvent(ctx context.Context, runID uint64, event GPTScriptEvent) (*Event, error) {
	if s == nil {
		return nil, nil
	}

	e := &Event{
		RunID:     runID,
		CreatedAt: time.Now(),
		Event:     datatypes.NewJSONType(event),
	}
	return e, s.db.WithContext(ctx).Clauses(clause.Returning{}).Create(e).Error
}

func (s *Store) ListEvents(ctx context.Context, runID uint64) ([]Event, error) {
	if s == nil {
		return nil, nil
	}

	var events []Event
	return events, s.db.WithContext(ctx).Where("run_id = ?", runID).Order("id ASC").Find(&events).Error
}

func (s *Store) GetEvent(ctx context.Context, runID, id uint64) (*Event, error) {
	if s == nil {
		return nil, nil
	}

	event := new(Event)
	return event, s.db.WithContext(ctx).Where("run_id = ? AND id = ?", runID, id).First(event).Error
}
