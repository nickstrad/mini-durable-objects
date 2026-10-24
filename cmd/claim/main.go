package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: claim <db-path> <agent-name>")
		os.Exit(2)
	}

	ctx := context.Background()

	db, err := sql.Open("sqlite", os.Args[1])
	if err != nil {
		panic(err)
	}
	defer db.Close()

	db.SetMaxOpenConns(1)

	// Pin every statement to one SQLite connection.
	conn, err := db.Conn(ctx)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Allow a short wait for another writer.
	if _, err := conn.ExecContext(
		ctx, "PRAGMA busy_timeout=2000",
	); err != nil {
		panic(err)
	}

	// Acquire the write reservation before reading or updating.
	if _, err := conn.ExecContext(
		ctx, "BEGIN IMMEDIATE",
	); err != nil {
		panic(err)
	}

	committed := false
	defer func() {
		if !committed {
			conn.ExecContext(ctx, "ROLLBACK")
		}
	}()

	// Only a pending task can be claimed.
	result, err := conn.ExecContext(ctx, `
		UPDATE task
		SET status = 'running'
		WHERE id = 1 AND status = 'pending'
	`)
	if err != nil {
		panic(err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		panic(err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		panic(err)
	}
	committed = true

	fmt.Printf(
		"%s: claimed=%t\n",
		os.Args[2],
		affected == 1,
	)
}
