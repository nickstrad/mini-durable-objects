package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	_ "modernc.org/sqlite"
)

var validID = regexp.MustCompile(
	`^[a-z0-9][a-z0-9-]{0,39}$`,
)

// Lesson 1: Object identity -> storage path.
func resolve(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", fmt.Errorf("invalid object ID: %q", id)
	}

	dir := filepath.Join("data", "objects", id)

	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}

	return filepath.Abs(
		filepath.Join(dir, "state.sqlite"),
	)
}

// Lesson 2: Open an object's persistent database.
func readState(id string) (string, error) {
	path, err := resolve(id)
	if err != nil {
		return "", err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return "", err
	}
	defer db.Close()

	// One connection per handle for this small lab.
	db.SetMaxOpenConns(1)

	// Initialize the object if necessary.
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS task (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			status TEXT NOT NULL DEFAULT 'pending'
		)
	`)
	if err != nil {
		return "", err
	}

	_, err = db.Exec(`
		INSERT OR IGNORE INTO task(id) VALUES(1)
	`)
	if err != nil {
		return "", err
	}

	var status string

	err = db.QueryRow(
		"SELECT status FROM task WHERE id = 1",
	).Scan(&status)

	return status, err
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: go run . resolve|state <object-id>")
		os.Exit(2)
	}

	id := os.Args[2]

	switch os.Args[1] {
	case "resolve":
		path, err := resolve(id)
		if err != nil {
			panic(err)
		}
		fmt.Println(path)

	case "state":
		status, err := readState(id)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s: %s\n", id, status)

	default:
		fmt.Println("Unknown command")
		os.Exit(2)
	}
}
