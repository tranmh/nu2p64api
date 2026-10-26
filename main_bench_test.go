package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// The package init() calls godotenv.Load() and log.Fatal()s without a .env in
// the working directory. Package-level variables are initialised before any
// init(), so this switches to a private temp directory holding a known .env
// (and the lumberjack main.log) before main.go's init() runs.
var benchEnvDir = setupBenchEnv()

const benchEnvBase = `API_SECRET=bench-api-secret
TOKEN_HOUR_LIFESPAN=1
JWT_USERNAME=anybody
JWT_PASSWORD=s3cr3t
LOG_LEVEL=info
SECRET_TOKEN=bench-secret-token
SSL_CERTFILE_FULLCHAIN=/nonexistent/fullchain.pem
SSL_PRIVATE_KEYFILE=/nonexistent/privkey.pem
yourMySQLdatabasepassword="bench"
basicAuthUsername=bench
basicAuthPassword="bench"
`

var (
	sinkString string
	sinkBool   bool
	sinkErr    error
	sinkBytes  []byte
)

func setupBenchEnv() string {
	dir, err := os.MkdirTemp("", "nu2p64api-bench-")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(benchEnvBase), 0o600); err != nil {
		panic(err)
	}
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	return dir
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	code := m.Run()
	os.RemoveAll(benchEnvDir)
	os.Exit(code)
}

// writeBenchEnv rewrites the .env with extra padding lines (comments and
// unrelated keys, as in a real deployment) and returns a restore function.
func writeBenchEnv(tb testing.TB, extraLines int) func() {
	var sb strings.Builder
	sb.WriteString(benchEnvBase)
	for i := 0; i < extraLines; i++ {
		if i%2 == 0 {
			fmt.Fprintf(&sb, "# padding comment line %d\n", i)
		} else {
			fmt.Fprintf(&sb, "PADDING_KEY_%d=\"padding value number %d\"\n", i, i)
		}
	}
	path := filepath.Join(benchEnvDir, ".env")
	if err := os.WriteFile(path, []byte(sb.String()), 0o600); err != nil {
		tb.Fatal(err)
	}
	return func() { _ = os.WriteFile(path, []byte(benchEnvBase), 0o600) }
}

// silenceOutput sends fmt.Printf (CJSON / AbortWithStatusJSON) and logrus to
// /dev/null for the duration of one benchmark function call.
func silenceOutput(tb testing.TB) func() {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		tb.Fatal(err)
	}
	oldStdout := os.Stdout
	oldLogOut := log.StandardLogger().Out
	os.Stdout = devNull
	log.SetOutput(io.Discard)
	return func() {
		os.Stdout = oldStdout
		log.SetOutput(oldLogOut)
		devNull.Close()
	}
}

var envSizes = []struct {
	name  string
	lines int
}{
	{"env=small", 0},
	{"env=medium", 100},
	{"env=large", 1000},
}

func newBenchContext(req *http.Request) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return c
}

// Middlewares call c.Next(), whose int8 index overflows when one test context
// is reused, so they run through an engine that resets its pooled context.
func newBenchEngine(mw gin.HandlerFunc, method, path string) *gin.Engine {
	e := gin.New()
	e.Use(mw)
	e.Handle(method, path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
	return e
}

type discardResponseWriter struct{ h http.Header }

func (w *discardResponseWriter) Header() http.Header         { return w.h }
func (w *discardResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (w *discardResponseWriter) WriteHeader(int)             {}

// P1: every authenticated request re-reads and re-parses .env.
func BenchmarkVerifyTokenController(b *testing.B) {
	for _, size := range envSizes {
		b.Run(size.name, func(b *testing.B) {
			defer writeBenchEnv(b, size.lines)()
			handler := verifyTokenController()
			req := httptest.NewRequest(http.MethodGet, "/api/persons/x", nil)
			req.Header.Set("Authorization", "Bearer bench-secret-token")
			c := newBenchContext(req)
			handler(c)
			if c.IsAborted() {
				b.Fatal("valid token was rejected")
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				handler(c)
			}
			sinkBool = c.IsAborted()
		})
	}
}

// P4 / t6: the JWT helpers re-read .env on each call.
func BenchmarkGenerateToken(b *testing.B) {
	for _, size := range envSizes {
		b.Run(size.name, func(b *testing.B) {
			defer writeBenchEnv(b, size.lines)()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkString, sinkErr = GenerateToken(1)
			}
			if sinkErr != nil {
				b.Fatal(sinkErr)
			}
		})
	}
}

func BenchmarkVerifyPassword(b *testing.B) {
	for _, size := range envSizes {
		b.Run(size.name, func(b *testing.B) {
			defer writeBenchEnv(b, size.lines)()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkBool = verifyPassword("anybody", "s3cr3t")
			}
			if !sinkBool {
				b.Fatal("expected valid credentials")
			}
		})
	}
}

func BenchmarkTokenValid(b *testing.B) {
	for _, size := range envSizes {
		b.Run(size.name, func(b *testing.B) {
			defer writeBenchEnv(b, size.lines)()
			token, err := GenerateToken(1)
			if err != nil {
				b.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/persons/x", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			c := newBenchContext(req)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkErr = tokenValid(c)
			}
			if sinkErr != nil {
				b.Fatal(sinkErr)
			}
		})
	}
}

func BenchmarkJwtAuthMiddleware(b *testing.B) {
	for _, size := range envSizes {
		b.Run(size.name, func(b *testing.B) {
			defer writeBenchEnv(b, size.lines)()
			token, err := GenerateToken(1)
			if err != nil {
				b.Fatal(err)
			}
			e := newBenchEngine(jwtAuthMiddleware(), http.MethodGet, "/api/persons/x")
			req := httptest.NewRequest(http.MethodGet, "/api/persons/x?token="+token, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent {
				b.Fatalf("valid token was rejected: %d", rec.Code)
			}
			w := &discardResponseWriter{h: http.Header{}}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				e.ServeHTTP(w, req)
			}
		})
	}
}

// P2 / t3: the phone regexp is compiled on every call. The phone length grows
// the matching cost; the compile cost stays as a constant per-call overhead,
// visible in the empty case.
func BenchmarkValidateDTOAddress(b *testing.B) {
	phone := func(groups int) string {
		var sb strings.Builder
		sb.WriteString("+49 (711) ")
		for i := 0; i < groups; i++ {
			if i > 0 {
				sb.WriteByte('-')
			}
			sb.WriteString("1234")
		}
		sb.WriteString(" ext. 42")
		return sb.String()
	}
	cases := []struct {
		name string
		addr DTOAddress
	}{
		{"phone=empty", DTOAddress{}},
		{"phone=small", DTOAddress{
			Email: "info@schachverein.de", WWW: "www.schachvereine.de",
			Phone_Mobile: phone(2), Phone_Home: phone(2), Phone_Work: phone(2),
		}},
		{"phone=medium", DTOAddress{
			Email: "info@schachverein.de", WWW: "www.schachvereine.de",
			Phone_Mobile: phone(32), Phone_Home: phone(32), Phone_Work: phone(32),
		}},
		{"phone=large", DTOAddress{
			Email: "info@schachverein.de", WWW: "www.schachvereine.de",
			Phone_Mobile: phone(512), Phone_Home: phone(512), Phone_Work: phone(512),
		}},
		{"invalidEmail", DTOAddress{Email: "not an email"}},
		{"invalidPhone", DTOAddress{Phone_Mobile: "call me maybe"}},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				sinkBool, sinkErr = validateDTOAddress(tc.addr)
			}
		})
	}
}

// P8: the middleware reads and logs the whole body on every request.
func BenchmarkLoggingMiddleware(b *testing.B) {
	for _, n := range []int{1 << 10, 64 << 10, 1 << 20} {
		b.Run(fmt.Sprintf("body=%dB", n), func(b *testing.B) {
			defer silenceOutput(b)()
			body := bytes.Repeat([]byte(`{"name":"Schachverein"},`), n/24+1)[:n]
			e := newBenchEngine(LoggingMiddleware(), http.MethodPut, "/api/addresses/x")
			req := httptest.NewRequest(http.MethodPut, "/api/addresses/x", bytes.NewReader(body))
			w := &discardResponseWriter{h: http.Header{}}
			b.SetBytes(int64(n))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// the middleware puts a fresh reader over the same bytes back
				// into req.Body, so every iteration reads the full body again
				e.ServeHTTP(w, req)
			}
		})
	}
}

func benchPayload(n int) gin.H {
	return gin.H{"uuid": "babe8313-f269-11ed-927b-005056054f4e", "remarks": strings.Repeat("x", n)}
}

func BenchmarkCJSON(b *testing.B) {
	for _, n := range []int{64, 4 << 10, 256 << 10} {
		b.Run(fmt.Sprintf("payload=%dB", n), func(b *testing.B) {
			defer silenceOutput(b)()
			payload := benchPayload(n)
			req := httptest.NewRequest(http.MethodGet, "/api/persons/x", nil)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c := newBenchContext(req)
				CJSON(c, http.StatusOK, payload)
			}
		})
	}
}

func BenchmarkAbortWithStatusJSON(b *testing.B) {
	for _, n := range []int{64, 4 << 10, 256 << 10} {
		b.Run(fmt.Sprintf("payload=%dB", n), func(b *testing.B) {
			defer silenceOutput(b)()
			payload := benchPayload(n)
			req := httptest.NewRequest(http.MethodGet, "/api/persons/x", nil)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c := newBenchContext(req)
				AbortWithStatusJSON(c, http.StatusBadRequest, payload)
			}
		})
	}
}

// t2: date parsing and formatting.
func BenchmarkCivilTimeUnmarshalJSON(b *testing.B) {
	for _, tc := range []struct{ name, in string }{
		{"date", `"2022-01-01"`},
		{"empty", `""`},
		{"null", `null`},
		{"invalid", `"invalid-date"`},
	} {
		b.Run(tc.name, func(b *testing.B) {
			in := []byte(tc.in)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var ct CivilTime
				sinkErr = ct.UnmarshalJSON(in)
			}
		})
	}
}

func BenchmarkCivilTimeMarshalJSON(b *testing.B) {
	var ct CivilTime
	if err := ct.UnmarshalJSON([]byte(`"2022-01-01"`)); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sinkBytes, sinkErr = ct.MarshalJSON()
	}
}

// t4: escaping used to build raw SQL strings.
func specialCharsInput(n int) string {
	unit := "O'Connor \xC2\x96 Lange \u009aAcker\xc2\x9e \\ "
	return strings.Repeat(unit, n/len(unit)+1)[:n]
}

func BenchmarkEscapeTick(b *testing.B) {
	for _, n := range []int{16, 1 << 10, 64 << 10} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			in := specialCharsInput(n)
			b.SetBytes(int64(n))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				sinkString = EscapeTick(in)
			}
		})
	}
}

func BenchmarkReplaceSpecialCharacters(b *testing.B) {
	for _, n := range []int{16, 1 << 10, 64 << 10} {
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			in := specialCharsInput(n)
			b.SetBytes(int64(n))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				sinkString = ReplaceSpecialCharacters(in)
			}
		})
	}
}

// t5: token from the query string or the Authorization header.
func BenchmarkExtractToken(b *testing.B) {
	for _, n := range []int{32, 1 << 10, 8 << 10} {
		tok := strings.Repeat("a", n)
		cases := []struct {
			name   string
			url    string
			header string
		}{
			{"query", "/api/x?token=" + tok, "Bearer other"},
			{"header", "/api/x", "Bearer " + tok},
			{"malformed", "/api/x", "Bearer " + tok + " extra"},
			{"emptyHeader", "/api/x", ""},
		}
		for _, tc := range cases {
			b.Run(fmt.Sprintf("%s/n=%d", tc.name, n), func(b *testing.B) {
				req := httptest.NewRequest(http.MethodGet, tc.url, nil)
				if tc.header != "" {
					req.Header.Set("Authorization", tc.header)
				}
				c := newBenchContext(req)
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					sinkString = extractToken(c)
				}
			})
		}
	}
}
