//go:build integration

package main

// Shared support for the integration tests of package main.
//
// The service talks MySQL (ifnull, backtick quoting, `\'` escapes) and opens
// its connection in main(), so the run's PostgreSQL service cannot stand in
// for the real database. The tests therefore exercise the real gin handlers
// over HTTP (httptest) against a scripted database/sql driver (fakeConnector
// from helpers_test.go, there is no sqlmock in go.mod) that answers each SQL
// statement by substring match and records every statement with its args.

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

const (
	itPersonUUID     = "babe8313-f269-11ed-927b-005056054f4e"
	itAddressUUID    = "6e358ea2-f26a-11ed-927b-005056054f4e"
	itClubUUID       = "7a1c0b52-0d1e-4c55-9d3e-0f4b5a6c7d8e"
	itFederationUUID = "6e25f2a5-f26a-11ed-927b-005056054f4e"
	itSportUUID1     = "11111111-2222-4333-8444-555555555555"
	itSportUUID2     = "66666666-7777-4888-9999-aaaaaaaaaaaa"
	itLicenseUUID    = "0b7c3d2e-1f4a-4b5c-8d6e-7f8091a2b3c4"
	itOfficialUUID   = "c4b3a291-8f7e-4d6c-9b5a-4e3d2c1b0a99"
	itOtherUUID      = "ffffffff-eeee-4ddd-8ccc-bbbbbbbbbbbb"
	itZeroUUID       = "00000000-0000-0000-0000-000000000000"
)

// ---------------------------------------------------------------------------
// scripted database

type scriptRule struct {
	contains string
	results  []fakeResult
	calls    int
}

type recordedCall struct {
	query string
	args  []driver.Value
}

type dbScript struct {
	mu    sync.Mutex
	rules []*scriptRule
	calls []recordedCall
}

// newDB installs a scripted fake database as the global db for the test.
func newDB(t *testing.T) *dbScript {
	t.Helper()
	s := &dbScript{}
	useFakeDB(t, s.handle)
	return s
}

// on answers statements containing substr. Results are consumed in order and
// the last one repeats. The first matching rule wins.
func (s *dbScript) on(substr string, results ...fakeResult) *dbScript {
	if len(results) == 0 {
		panic("on() needs at least one result")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = append(s.rules, &scriptRule{contains: substr, results: results})
	return s
}

func (s *dbScript) handle(query string, args []driver.NamedValue) fakeResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	vals := make([]driver.Value, len(args))
	for i, a := range args {
		vals[i] = a.Value
	}
	s.calls = append(s.calls, recordedCall{query: query, args: vals})
	for _, r := range s.rules {
		if strings.Contains(query, r.contains) {
			i := r.calls
			if i >= len(r.results) {
				i = len(r.results) - 1
			}
			r.calls++
			return r.results[i]
		}
	}
	return fakeResult{err: fmt.Errorf("aqaa fake db: unexpected query: %s", query)}
}

// all returns every recorded statement containing substr.
func (s *dbScript) all(substr string) []recordedCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []recordedCall
	for _, c := range s.calls {
		if strings.Contains(c.query, substr) {
			out = append(out, c)
		}
	}
	return out
}

// one returns the only recorded statement containing substr.
func (s *dbScript) one(t *testing.T, substr string) recordedCall {
	t.Helper()
	got := s.all(substr)
	require.Lenf(t, got, 1, "expected exactly one statement containing %q, got %d: %v", substr, len(got), s.queries())
	return got[0]
}

func (s *dbScript) queries() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.calls))
	for i, c := range s.calls {
		out[i] = c.query
	}
	return out
}

func colsFor(n int) []string {
	cols := make([]string, n)
	for i := range cols {
		cols[i] = fmt.Sprintf("c%d", i)
	}
	return cols
}

// rowOf is a result with exactly one row.
func rowOf(vals ...driver.Value) fakeResult {
	return fakeResult{cols: colsFor(len(vals)), rows: [][]driver.Value{vals}}
}

// rowsOf is a result with ncols columns and the given rows.
func rowsOf(ncols int, rows ...[]driver.Value) fakeResult {
	return fakeResult{cols: colsFor(ncols), rows: rows}
}

// noRows is an empty result (QueryRow yields sql.ErrNoRows).
func noRows(ncols int) fakeResult { return fakeResult{cols: colsFor(ncols)} }

// dbFail makes the statement itself fail.
func dbFail(msg string) fakeResult { return fakeResult{err: errors.New(msg)} }

// affected is a successful Exec.
func affected(n int64) fakeResult { return fakeResult{affected: n} }

var sqlLineComment = regexp.MustCompile(`--[ \t][^\n]*`)

// stripSQLComments removes MySQL `-- ` line comments.
func stripSQLComments(q string) string { return sqlLineComment.ReplaceAllString(q, "") }

// ---------------------------------------------------------------------------
// HTTP

// newAPIRouter mirrors the router built in main(): recovery, request logging
// and the /api group protected by verifyTokenController.
func newAPIRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(LoggingMiddleware())

	authorized := router.Group("/api", verifyTokenController())

	authorized.PUT("/regions/:reg_uuid", putDTORegion)

	authorized.GET("/federations/:fed_uuid", getDTOFederation)
	authorized.PUT("/federations/:fed_uuid", putDTOFederation)
	authorized.DELETE("/federations/:fed_uuid", deleteDTOFederation)

	authorized.GET("/clubs/:club_uuid", getDTOClub)
	authorized.PUT("/clubs/:club_uuid", putDTOClub)
	authorized.DELETE("/clubs/:club_uuid", deleteDTOClub)

	authorized.GET("/addresses/:addr_uuid", getDTOAddress)
	authorized.PUT("/addresses/:addr_uuid", putDTOAddress)
	authorized.DELETE("/addresses/:addr_uuid", deleteDTOAddress)

	authorized.GET("/persons/:pers_uuid", getDTOPerson)
	authorized.PUT("/persons/:pers_uuid", putDTOPerson)
	authorized.DELETE("/persons/:pers_uuid", deleteDTOPerson)

	authorized.GET("/club-members/:clubmem_uuid", getDTOClubMember)
	authorized.PUT("/club-members/:clubmem_uuid", putDTOClubMember)
	authorized.DELETE("/club-members/:clubmem_uuid", deleteDTOClubMember)

	authorized.GET("/club-officials/:official_uuid", getDTOClubOfficial)
	authorized.PUT("/club-officials/:official_uuid", putDTOClubOfficial)
	authorized.DELETE("/club-officials/:official_uuid", deleteDTOClubOfficial)

	authorized.GET("/player-licences/:license_uuid", getDTOPlayerLicense)
	authorized.PUT("/player-licences/:license_uuid", putDTOPlayerLicense)
	authorized.DELETE("/player-licences/:license_uuid", deleteDTOPlayerLicense)

	return router
}

// callAPI sends an authorized request through the API router.
func callAPI(method, path, body string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Authorization", "Bearer "+testSecretToken)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	newAPIRouter().ServeHTTP(w, req)
	return w
}

func uuidMust(s string) uuid.UUID { return uuid.MustParse(s) }

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func decodeBody[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoErrorf(t, json.Unmarshal(w.Body.Bytes(), &v), "body: %s", w.Body.String())
	return v
}

// ---------------------------------------------------------------------------
// .env

// withEnv rewrites the test .env (read on every request by the code under
// test) with the given overrides and restores it when the test ends.
func withEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	path := filepath.Join(testEnvDir, ".env")
	orig, err := os.ReadFile(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.WriteFile(path, orig, 0o600) })

	env, err := godotenv.Unmarshal(string(orig))
	require.NoError(t, err)
	for k, v := range overrides {
		env[k] = v
	}
	require.NoError(t, godotenv.Write(env, path))
}
