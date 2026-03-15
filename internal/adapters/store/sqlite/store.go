package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	gormsqlite "github.com/glebarez/sqlite"
	"github.com/yoke233/ai-workflow/internal/core"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Store implements core.Store backed by SQLite.
type Store struct {
	db  *sql.DB
	orm *gorm.DB
}

const startupDBTimeout = 6 * time.Second

// New opens (or creates) a SQLite database at path and ensures schema exists via GORM AutoMigrate.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), startupDBTimeout)
	defer cancel()

	maxOpenConns := sqliteMaxOpenConns(path)
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxOpenConns)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, startupDBError(path, "ping sqlite", err)
	}
	// Enable WAL mode and foreign keys.
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, startupDBError(path, fmt.Sprintf("exec %s", pragma), err)
		}
	}
	orm, err := gorm.Open(gormsqlite.Dialector{
		DriverName: "sqlite",
		Conn:       db,
	}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open gorm sqlite %s: %w", path, err)
	}
	if err := autoMigrate(ctx, orm); err != nil {
		db.Close()
		return nil, startupDBError(path, "auto migrate", err)
	}

	return &Store{db: db, orm: orm}, nil
}

func sqliteMaxOpenConns(path string) int {
	normalized := strings.ToLower(strings.TrimSpace(path))
	// Plain in-memory SQLite databases are isolated per connection, so keep them
	// on a single connection to preserve the existing test/runtime behavior.
	if normalized == ":memory:" || strings.Contains(normalized, "mode=memory") {
		return 1
	}

	maxOpenConns := runtime.GOMAXPROCS(0)
	if maxOpenConns < 4 {
		maxOpenConns = 4
	}
	if maxOpenConns > 8 {
		maxOpenConns = 8
	}
	return maxOpenConns
}

func startupDBError(path string, op string, err error) error {
	if err == nil {
		return nil
	}
	msg := fmt.Sprintf("%s %s: %v", op, path, err)
	if errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(err.Error()), "database is locked") {
		return fmt.Errorf("%s; database may be locked by another ai-flow process, stop old processes and remove %s-shm/%s-wal if needed", msg, path, path)
	}
	return fmt.Errorf("%s", msg)
}

func (s *Store) cloneWithORM(orm *gorm.DB) *Store {
	if s == nil {
		return nil
	}
	return &Store{
		db:  s.db,
		orm: orm,
	}
}

// InTx runs fn inside a single database transaction using a cloned store
// bound to the transaction-scoped gorm handle.
func (s *Store) InTx(ctx context.Context, fn func(store core.Store) error) error {
	if s == nil || s.orm == nil {
		return fmt.Errorf("store is not initialized")
	}
	return s.orm.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(s.cloneWithORM(tx))
	})
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
