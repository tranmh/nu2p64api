package main

// Test support for package main:
//
//   - main.go's init() calls godotenv.Load() and log.Fatal()s when no .env file
//     is present in the working directory. Package level variables are
//     initialised before any init() function runs, so testEnvDir below creates
//     a temporary directory with a deterministic .env file and changes into it
//     before main.go's init() executes.
//   - fakeConnector is a tiny in-memory database/sql driver used to replace the
//     global db handle (no sqlmock dependency is available in go.mod).

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

const (
	testAPISecret   = "test-api-secret"
	testSecretToken = "test-secret-token"
	testJWTUser     = "anybody"
	testJWTPassword = "s3cr3t"
)

var testEnvDir = setupTestEnv()

func setupTestEnv() string {
	dir, err := os.MkdirTemp("", "nu2p64api-test-")
	if err != nil {
		panic(err)
	}
	env := "API_SECRET=" + testAPISecret + "\n" +
		"TOKEN_HOUR_LIFESPAN=1\n" +
		"JWT_USERNAME=" + testJWTUser + "\n" +
		"JWT_PASSWORD=" + testJWTPassword + "\n" +
		"LOG_LEVEL=error\n" +
		"SECRET_TOKEN=" + testSecretToken + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0o600); err != nil {
		panic(err)
	}
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	return dir
}

func TestMain(m *testing.M) {
	code := m.Run()
	_ = os.RemoveAll(testEnvDir)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// fake database/sql driver

type fakeResult struct {
	cols     []string
	rows     [][]driver.Value
	err      error // returned by Query/Exec itself
	rowsErr  error // returned by rows.Next after all rows were delivered
	affected int64 // rows affected for Exec
}

type fakeHandler func(query string, args []driver.NamedValue) fakeResult

type fakeConnector struct {
	h       fakeHandler
	mu      sync.Mutex
	queries []string
}

func (c *fakeConnector) Connect(context.Context) (driver.Conn, error) {
	return &fakeConn{c: c}, nil
}

func (c *fakeConnector) Driver() driver.Driver { return fakeDriver{} }

func (c *fakeConnector) run(query string, args []driver.NamedValue) fakeResult {
	c.mu.Lock()
	c.queries = append(c.queries, query)
	c.mu.Unlock()
	return c.h(query, args)
}

func (c *fakeConnector) Queries() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.queries...)
}

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("fake driver: use sql.OpenDB with a fakeConnector")
}

type fakeConn struct{ c *fakeConnector }

func (fc *fakeConn) Prepare(query string) (driver.Stmt, error) {
	return &fakeStmt{fc: fc, query: query}, nil
}
func (fc *fakeConn) Close() error { return nil }
func (fc *fakeConn) Begin() (driver.Tx, error) {
	return nil, errors.New("fake driver: transactions not supported")
}

func (fc *fakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	r := fc.c.run(query, args)
	if r.err != nil {
		return nil, r.err
	}
	return &fakeRows{r: r}, nil
}

func (fc *fakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	r := fc.c.run(query, args)
	if r.err != nil {
		return nil, r.err
	}
	return driver.RowsAffected(r.affected), nil
}

type fakeStmt struct {
	fc    *fakeConn
	query string
}

func (s *fakeStmt) Close() error  { return nil }
func (s *fakeStmt) NumInput() int { return -1 }

func toNamed(args []driver.Value) []driver.NamedValue {
	named := make([]driver.NamedValue, len(args))
	for i, a := range args {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: a}
	}
	return named
}

func (s *fakeStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.fc.ExecContext(context.Background(), s.query, toNamed(args))
}

func (s *fakeStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.fc.QueryContext(context.Background(), s.query, toNamed(args))
}

type fakeRows struct {
	r fakeResult
	i int
}

func (r *fakeRows) Columns() []string { return r.r.cols }
func (r *fakeRows) Close() error      { return nil }
func (r *fakeRows) Next(dest []driver.Value) error {
	if r.i >= len(r.r.rows) {
		if r.r.rowsErr != nil {
			return r.r.rowsErr
		}
		return io.EOF
	}
	copy(dest, r.r.rows[r.i])
	r.i++
	return nil
}

// useFakeDB replaces the global db with a fake for the duration of the test.
func useFakeDB(t *testing.T, h fakeHandler) *fakeConnector {
	t.Helper()
	conn := &fakeConnector{h: h}
	fake := sql.OpenDB(conn)
	old := db
	db = fake
	t.Cleanup(func() {
		_ = fake.Close()
		db = old
	})
	return conn
}
