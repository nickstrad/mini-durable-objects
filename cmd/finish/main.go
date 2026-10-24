package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func finish(path string, injectFailure bool) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	// All changes belong to one transaction.
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Rolls back unless Commit succeeds.
	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE task
		SET status='completed'
		WHERE id=1 AND status='running'
	`)
	if err != nil {
		return err
	}

	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("task is not running")
	}

	_, err = tx.Exec(`
		INSERT INTO events(kind)
		VALUES('completed')
	`)
	if err != nil {
		return err
	}

	// Demonstrate failure after both mutations.
	if injectFailure {
		return errors.New("simulated failure before commit")
	}

	return tx.Commit()
}

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Println("Usage: finish <db-path> [--fail]")
		os.Exit(2)
	}

	fail := len(os.Args) == 3 && os.Args[2] == "--fail"

	if err := finish(os.Args[1], fail); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println("Task and event committed")
}
