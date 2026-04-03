package db

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaSQL string

const dropSchemaSQL = `
DROP TRIGGER IF EXISTS messages_fts_insert;
DROP TRIGGER IF EXISTS messages_fts_delete;
DROP TRIGGER IF EXISTS messages_fts_update;
DROP TABLE IF EXISTS messages_fts;
DROP TABLE IF EXISTS brain_links;
DROP TABLE IF EXISTS brain_documents;
DROP TABLE IF EXISTS context_reports;
DROP TABLE IF EXISTS sub_calls;
DROP TABLE IF EXISTS tool_executions;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS index_state;
DROP TABLE IF EXISTS conversations;
DROP TABLE IF EXISTS projects;
`

// Init recreates the full SQLite schema from scratch.
// WARNING: This drops all existing tables first. Use InitIfNeeded for idempotent setup.
func Init(ctx context.Context, db *sql.DB) error {
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema init transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, dropSchemaSQL); err != nil {
		return fmt.Errorf("drop existing schema: %w", err)
	}
	if _, err := tx.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema init transaction: %w", err)
	}
	return nil
}

// InitIfNeeded creates the schema only if the core tables do not yet exist.
// Safe to call repeatedly — returns (true, nil) if schema was created,
// (false, nil) if it already existed.
func InitIfNeeded(ctx context.Context, db *sql.DB) (created bool, err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var count int
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='projects'`,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check existing schema: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin schema init transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, schemaSQL); err != nil {
		return false, fmt.Errorf("apply schema: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit schema init transaction: %w", err)
	}
	return true, nil
}
