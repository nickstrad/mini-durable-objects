package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/mattn/go-sqlite3"
)

const (
	queryTimeout = 2 * time.Second
	maxQueryRows = 100

	// mattn/go-sqlite3 leaves SQLITE_RECURSIVE (WITH RECURSIVE) commented out.
	sqliteRecursive = 33
)

// authorizeQuery is SQLite's authorizer for client SQL. SQLite calls it while
// preparing a statement, once per action (read a column, insert into a table,
// run a PRAGMA, ...), and refuses to prepare the statement if any action is
// denied. Because SQLite does the parsing, tricks in the SQL text can't slip
// past it, unlike a string check.
//
// It allows reads, row writes, and creating, altering, or dropping tables and
// indexes, so clients manage their own schema. Each object has its own file,
// so a client can only change its own data. Everything else is denied:
// PRAGMA (it could turn off safety settings), ATTACH (it could open other
// files, such as another object's database), transactions (the store owns
// them), and triggers, views, virtual tables, and temp objects (not needed
// yet). SQLite itself refuses direct writes to its sqlite_* tables.
//
// The check runs only when a statement is prepared, so do not turn on the
// driver's statement cache (_stmt_cache_size): a cached statement is not checked again.
//
// objectDB.authorize calls it only while client SQL runs, so the store's own
// BEGIN, COMMIT, and ROLLBACK are not checked.
func authorizeQuery(action int, arg1, arg2, dbName string) int {
	switch action {
	case sqlite3.SQLITE_SELECT, sqlite3.SQLITE_READ, sqlite3.SQLITE_FUNCTION, sqliteRecursive:
		return sqlite3.SQLITE_OK
	case sqlite3.SQLITE_INSERT, sqlite3.SQLITE_UPDATE, sqlite3.SQLITE_DELETE,
		sqlite3.SQLITE_CREATE_TABLE, sqlite3.SQLITE_CREATE_INDEX,
		sqlite3.SQLITE_DROP_TABLE, sqlite3.SQLITE_DROP_INDEX, sqlite3.SQLITE_ALTER_TABLE,
		sqlite3.SQLITE_REINDEX: // CREATE INDEX checks this while filling the index.
		// Schema changes also write SQLite's schema table, which arrives here
		// as an INSERT, UPDATE, or DELETE on sqlite_master.
		return sqlite3.SQLITE_OK
	}
	return sqlite3.SQLITE_DENY
}

// objectDB is an object's open database. checkSQL turns the authorizer on
// while client SQL runs. It is atomic because database/sql may roll back a
// canceled transaction from its own goroutine.
type objectDB struct {
	*sql.DB
	checkSQL atomic.Bool
}

func (o *objectDB) authorize(action int, arg1, arg2, dbName string) int {
	if !o.checkSQL.Load() {
		return sqlite3.SQLITE_OK
	}
	return authorizeQuery(action, arg1, arg2, dbName)
}

// openDB opens path with the authorizer installed once per connection.
// (Installing it per query would leak a driver handle each time until the
// connection closes.)
func openDB(path string) (*objectDB, error) {
	o := &objectDB{}
	d := &sqlite3.SQLiteDriver{ConnectHook: func(c *sqlite3.SQLiteConn) error {
		c.RegisterAuthorizer(o.authorize)
		return nil
	}}
	o.DB = sql.OpenDB(connector{driver: d, path: path})
	o.SetMaxOpenConns(1)
	return o, nil
}

// connector opens path with its own driver, so each object gets its own ConnectHook.
type connector struct {
	driver *sqlite3.SQLiteDriver
	path   string
}

func (c connector) Connect(context.Context) (driver.Conn, error) { return c.driver.Open(c.path) }
func (c connector) Driver() driver.Driver                        { return c.driver }

func (s SQLite) Query(ctx context.Context, id, query string, params []any) (QueryResult, error) {
	out, err := s.query(ctx, id, query, params)
	return out, wrapErr("store.Query", id, err)
}

func (s SQLite) query(ctx context.Context, id, query string, params []any) (QueryResult, error) {
	var out QueryResult
	err := s.do(ctx, id, func(db *objectDB) error {
		// The budget starts once the object runs the query, not while it waits in line.
		ctx, cancel := context.WithTimeout(ctx, queryTimeout)
		defer cancel()
		var err error
		out, err = runQuery(ctx, db, query, params)
		return err
	})
	return out, err
}

// runQuery runs client SQL in its own transaction. All rows are read before
// committing, so a write only sticks if the whole result fit under maxQueryRows.
func runQuery(ctx context.Context, db *objectDB, query string, params []any) (QueryResult, error) {
	out := QueryResult{Rows: [][]any{}}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	db.checkSQL.Store(true)
	defer db.checkSQL.Store(false) // Runs before the deferred Rollback.

	rows, err := tx.QueryContext(ctx, query, params...)
	if err != nil {
		return out, fmt.Errorf("%w: %w", errBadQuery, err)
	}
	defer rows.Close()
	if out.Columns, err = rows.Columns(); err != nil {
		return out, err
	}
	if out.Columns == nil {
		out.Columns = []string{} // Statements like INSERT have no columns; keep JSON an array.
	}
	targets := make([]any, len(out.Columns))
	for rows.Next() {
		if len(out.Rows) == maxQueryRows {
			return out, fmt.Errorf("%w: result exceeds %d rows; use LIMIT", errBadQuery, maxQueryRows)
		}
		values := make([]any, len(out.Columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return out, err
		}
		out.Rows = append(out.Rows, values)
	}
	if err := rows.Err(); err != nil {
		return out, fmt.Errorf("%w: %w", errBadQuery, err)
	}
	db.checkSQL.Store(false)
	return out, tx.Commit()
}
