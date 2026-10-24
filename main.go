package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var validID = regexp.MustCompile(
	`^[a-z0-9][a-z0-9-]{0,39}$`,
)

// resolve maps a logical object ID to its storage path.
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

func main() {
	if len(os.Args) != 3 || os.Args[1] != "resolve" {
		fmt.Println("Usage: go run . resolve <object-id>")
		os.Exit(2)
	}

	path, err := resolve(os.Args[2])
	if err != nil {
		panic(err)
	}

	fmt.Println(path)
}
