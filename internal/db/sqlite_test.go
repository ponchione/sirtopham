//go:build sqlite_fts5
// +build sqlite_fts5

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestOpenDBAppliesPragmas(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "nested", "test.db")

	db, err := OpenDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenDB returned error: %v", err)
	}
	defer db.Close()

	assertPragmaValue(t, db, "PRAGMA journal_mode;", expectedJournalMode)
	assertPragmaValue(t, db, "PRAGMA busy_timeout;", expectedBusyTimeout)
	assertPragmaValue(t, db, "PRAGMA foreign_keys;", expectedForeignKeys)
	assertPragmaValue(t, db, "PRAGMA synchronous;", expectedSynchronous)
}

func TestOpenDBConcurrentReadWriteWAL(t *testing.T) {
	ctx := context.Background()
	db, err := OpenDB(ctx, filepath.Join(t.TempDir(), "wal.db"))
	if err != nil {
		t.Fatalf("OpenDB returned error: %v", err)
	}
	defer db.Close()

	if _, err := db.ExecContext(ctx, `CREATE TABLE wal_test (id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT NOT NULL);`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	readerSawData := make(chan struct{}, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if _, err := db.ExecContext(ctx, `INSERT INTO wal_test(value) VALUES (?)`, fmt.Sprintf("value-%d", i)); err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			var count int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM wal_test;`).Scan(&count); err != nil {
				errCh <- err
				return
			}
			if count > 0 {
				select {
				case readerSawData <- struct{}{}:
				default:
				}
				return
			}
		}
		errCh <- errors.New("reader never observed written rows")
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err == nil {
			continue
		}
		if strings.Contains(strings.ToLower(err.Error()), "database is locked") {
			t.Fatalf("unexpected database lock under WAL: %v", err)
		}
		t.Fatalf("concurrent read/write failed: %v", err)
	}

	select {
	case <-readerSawData:
	default:
		t.Fatal("reader did not observe committed data")
	}
}

func TestOpenDBCloseAndReopenPreservesData(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "persist.db")

	db, err := OpenDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenDB returned error: %v", err)
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE shutdown_test (id INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT NOT NULL);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO shutdown_test(value) VALUES ('persisted');`); err != nil {
		t.Fatalf("insert row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	reopened, err := OpenDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen returned error: %v", err)
	}
	defer reopened.Close()

	var count int
	if err := reopened.QueryRowContext(ctx, `SELECT COUNT(*) FROM shutdown_test;`).Scan(&count); err != nil {
		t.Fatalf("query persisted row: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

func assertPragmaValue(t *testing.T, db *sql.DB, query string, want any) {
	t.Helper()

	switch expected := want.(type) {
	case string:
		var got string
		if err := db.QueryRow(query).Scan(&got); err != nil {
			t.Fatalf("query %q failed: %v", query, err)
		}
		if strings.ToLower(got) != expected {
			t.Fatalf("pragma %q = %q, want %q", query, got, expected)
		}
	case int:
		var got int
		if err := db.QueryRow(query).Scan(&got); err != nil {
			t.Fatalf("query %q failed: %v", query, err)
		}
		if got != expected {
			t.Fatalf("pragma %q = %d, want %d", query, got, expected)
		}
	default:
		t.Fatalf("unsupported pragma assertion type %T", want)
	}
}
