//go:build integration

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// t5: verifyTokenController on the /api group (B2, B3)

func TestIntegrationVerifyTokenController(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var handlerCalls int
	var afterMiddlewareCalls int
	router := gin.New()
	router.Use(gin.Recovery())
	api := router.Group("/api", verifyTokenController(), func(c *gin.Context) {
		// runs only if verifyTokenController did not abort, even though it never calls c.Next()
		afterMiddlewareCalls++
	})
	api.GET("/probe", func(c *gin.Context) {
		handlerCalls++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	const malformed = "Authentication header not present or malformed"
	const wrong = "Authentication token is not correct"

	tests := []struct {
		name       string
		header     *string
		wantStatus int
		wantMsg    string
	}{
		{name: "missing header", header: nil, wantStatus: 401, wantMsg: malformed},
		{name: "empty header", header: strPtr(""), wantStatus: 401, wantMsg: malformed},
		{name: "token without bearer prefix", header: strPtr(testSecretToken), wantStatus: 401, wantMsg: malformed},
		{name: "basic scheme", header: strPtr("Basic " + testSecretToken), wantStatus: 401, wantMsg: malformed},
		{name: "lower case bearer", header: strPtr("bearer " + testSecretToken), wantStatus: 401, wantMsg: malformed},
		{name: "bearer without token", header: strPtr("Bearer"), wantStatus: 401, wantMsg: malformed},
		{name: "wrong token", header: strPtr("Bearer not-the-token"), wantStatus: 401, wantMsg: wrong},
		{name: "token with suffix", header: strPtr("Bearer " + testSecretToken + "x"), wantStatus: 401, wantMsg: wrong},
		{name: "token prefix only", header: strPtr("Bearer " + testSecretToken[:4]), wantStatus: 401, wantMsg: wrong},
		{name: "correct token", header: strPtr("Bearer " + testSecretToken), wantStatus: 200},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handlerCalls, afterMiddlewareCalls = 0, 0
			req, err := http.NewRequest(http.MethodGet, srv.URL+"/api/probe", nil)
			require.NoError(t, err)
			if tc.header != nil {
				req.Header.Set("Authorization", *tc.header)
			}
			resp, err := srv.Client().Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
			if tc.wantStatus == 200 {
				assert.Equal(t, 1, handlerCalls)
				assert.Equal(t, 1, afterMiddlewareCalls)
				assert.JSONEq(t, `{"ok":true}`, string(body))
				return
			}
			assert.Equal(t, 0, handlerCalls, "handler must not run after abort")
			assert.Equal(t, 0, afterMiddlewareCalls, "chain must be aborted")
			assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
			var msg map[string]string
			require.NoError(t, json.Unmarshal(body, &msg), string(body))
			assert.Equal(t, map[string]string{"Message": tc.wantMsg}, msg)
		})
	}
}

func strPtr(s string) *string { return &s }

func TestIntegrationVerifyTokenControllerReadsEnvPerRequest(t *testing.T) {
	newDB(t) // no database access expected
	w := callAPI(http.MethodPut, "/api/regions/"+itOtherUUID, `{}`)
	assert.Equal(t, http.StatusNoContent, w.Code)

	withEnv(t, map[string]string{"SECRET_TOKEN": "rotated-token"})

	w = callAPI(http.MethodPut, "/api/regions/"+itOtherUUID, `{}`)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"Message":"Authentication token is not correct"}`, w.Body.String())

	req := httptest.NewRequest(http.MethodPut, "/api/regions/"+itOtherUUID, strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer rotated-token")
	rec := httptest.NewRecorder()
	newAPIRouter().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestIntegrationAPIRoutesRejectUnauthenticated(t *testing.T) {
	db := newDB(t)
	paths := []string{"regions", "federations", "clubs", "addresses", "persons", "club-members", "club-officials", "player-licences"}
	for _, p := range paths {
		for _, m := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			if p == "regions" && m != http.MethodPut {
				continue
			}
			t.Run(m+" "+p, func(t *testing.T) {
				req := httptest.NewRequest(m, "/api/"+p+"/"+itOtherUUID, strings.NewReader(`{}`))
				w := httptest.NewRecorder()
				newAPIRouter().ServeHTTP(w, req)
				assert.Equal(t, http.StatusUnauthorized, w.Code)
				assert.JSONEq(t, `{"Message":"Authentication header not present or malformed"}`, w.Body.String())
			})
		}
	}
	assert.Empty(t, db.queries(), "no handler may touch the database without authentication")
}

// ---------------------------------------------------------------------------
// t6: loginUser / loginCheck / GenerateToken / jwtAuthMiddleware (B5, B6, B7)

func newJWTRouter(protectedCalls *int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(gin.Recovery())
	public := router.Group("/public")
	public.POST("/register", registerLoginUser)
	public.POST("/login", loginUser)
	protected := router.Group("/jwt")
	protected.Use(jwtAuthMiddleware())
	protected.GET("/ping", func(c *gin.Context) {
		*protectedCalls++
		c.String(http.StatusOK, "pong")
	})
	return router
}

func postJSON(r http.Handler, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func getWithToken(r http.Handler, path, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func signHS(t *testing.T, method jwt.SigningMethod, secret string, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString([]byte(secret))
	require.NoError(t, err)
	return s
}

func TestIntegrationLoginFlow(t *testing.T) {
	var calls int
	router := newJWTRouter(&calls)

	before := time.Now().Unix()
	w := postJSON(router, "/public/login", `{"username":"`+testJWTUser+`","password":"`+testJWTPassword+`"}`)
	after := time.Now().Unix()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	token := resp["token"]
	require.NotEmpty(t, token)

	// the token is an HS256 JWT signed with API_SECRET (B6)
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(tok *jwt.Token) (interface{}, error) {
		return []byte(testAPISecret), nil
	})
	require.NoError(t, err)
	assert.True(t, parsed.Valid)
	assert.Equal(t, "HS256", parsed.Header["alg"])
	assert.Equal(t, true, claims["authorized"])
	assert.Equal(t, float64(1), claims["user_id"])
	exp, ok := claims["exp"].(float64)
	require.True(t, ok)
	assert.GreaterOrEqual(t, int64(exp), before+3600, "TOKEN_HOUR_LIFESPAN=1")
	assert.LessOrEqual(t, int64(exp), after+3600)

	t.Run("token validates as bearer header", func(t *testing.T) {
		calls = 0
		w := getWithToken(router, "/jwt/ping", "Bearer "+token)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "pong", w.Body.String())
		assert.Equal(t, 1, calls)
	})
	t.Run("token validates as query parameter", func(t *testing.T) {
		calls = 0
		w := getWithToken(router, "/jwt/ping?token="+token, "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, 1, calls)
	})
}

func TestIntegrationLoginRejects(t *testing.T) {
	var calls int
	router := newJWTRouter(&calls)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  string // exact error message, "" = only check presence
	}{
		{name: "wrong password", body: `{"username":"` + testJWTUser + `","password":"nope"}`, wantStatus: 400, wantError: "username or password is incorrect."},
		{name: "wrong username", body: `{"username":"root","password":"` + testJWTPassword + `"}`, wantStatus: 400, wantError: "username or password is incorrect."},
		{name: "case sensitive username", body: `{"username":"ANYBODY","password":"` + testJWTPassword + `"}`, wantStatus: 400, wantError: "username or password is incorrect."},
		{name: "missing password", body: `{"username":"` + testJWTUser + `"}`, wantStatus: 400},
		{name: "missing username", body: `{"password":"` + testJWTPassword + `"}`, wantStatus: 400},
		{name: "empty password", body: `{"username":"` + testJWTUser + `","password":""}`, wantStatus: 400},
		{name: "empty object", body: `{}`, wantStatus: 400},
		{name: "not json", body: `username=anybody`, wantStatus: 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := postJSON(router, "/public/login", tc.body)
			assert.Equal(t, tc.wantStatus, w.Code)
			var resp map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), w.Body.String())
			assert.NotContains(t, resp, "token")
			require.Contains(t, resp, "error")
			if tc.wantError != "" {
				assert.Equal(t, tc.wantError, resp["error"])
			}
		})
	}
}

func TestIntegrationLoginInvalidTokenLifespan(t *testing.T) {
	for _, lifespan := range []string{"", "one", "1.5", "1h"} {
		t.Run("TOKEN_HOUR_LIFESPAN="+lifespan, func(t *testing.T) {
			withEnv(t, map[string]string{"TOKEN_HOUR_LIFESPAN": lifespan})

			tok, err := GenerateToken(1)
			assert.Error(t, err)
			assert.Empty(t, tok)

			tok, err = loginCheck(testJWTUser, testJWTPassword)
			assert.Error(t, err)
			assert.Empty(t, tok)

			var calls int
			w := postJSON(newJWTRouter(&calls), "/public/login", `{"username":"`+testJWTUser+`","password":"`+testJWTPassword+`"}`)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.NotContains(t, w.Body.String(), "token")
		})
	}
}

func TestIntegrationJWTAuthMiddleware(t *testing.T) {
	var calls int
	router := newJWTRouter(&calls)

	valid := jwt.MapClaims{"authorized": true, "user_id": 1, "exp": time.Now().Add(time.Hour).Unix()}
	expired := jwt.MapClaims{"authorized": true, "user_id": 1, "exp": int64(1000000000)} // 2001-09-09
	noneToken, err := jwt.NewWithClaims(jwt.SigningMethodNone, valid).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	tests := []struct {
		name       string
		header     string
		query      string
		wantStatus int
	}{
		{name: "no token", wantStatus: 401},
		{name: "garbage token", header: "Bearer not.a.jwt", wantStatus: 401},
		{name: "expired token", header: "Bearer " + signHS(t, jwt.SigningMethodHS256, testAPISecret, expired), wantStatus: 401},
		{name: "wrong secret", header: "Bearer " + signHS(t, jwt.SigningMethodHS256, "other-secret", valid), wantStatus: 401},
		{name: "signing method none", header: "Bearer " + noneToken, wantStatus: 401},
		{name: "valid token without bearer scheme", header: signHS(t, jwt.SigningMethodHS256, testAPISecret, valid), wantStatus: 401},
		{name: "valid HS256 token", header: "Bearer " + signHS(t, jwt.SigningMethodHS256, testAPISecret, valid), wantStatus: 200},
		{name: "valid HS512 token is HMAC too", header: "Bearer " + signHS(t, jwt.SigningMethodHS512, testAPISecret, valid), wantStatus: 200},
		{name: "valid token as query parameter", query: signHS(t, jwt.SigningMethodHS256, testAPISecret, valid), wantStatus: 200},
		{name: "expired query token wins over valid header", query: signHS(t, jwt.SigningMethodHS256, testAPISecret, expired),
			header: "Bearer " + signHS(t, jwt.SigningMethodHS256, testAPISecret, valid), wantStatus: 401},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			calls = 0
			path := "/jwt/ping"
			if tc.query != "" {
				path += "?token=" + tc.query
			}
			w := getWithToken(router, path, tc.header)
			assert.Equal(t, tc.wantStatus, w.Code)
			if tc.wantStatus == 401 {
				assert.Equal(t, "Unauthorized", w.Body.String())
				assert.Equal(t, 0, calls, "handler must not run")
			} else {
				assert.Equal(t, 1, calls)
			}
		})
	}
}

func TestIntegrationRegisterLoginUser(t *testing.T) {
	var calls int
	router := newJWTRouter(&calls)

	w := postJSON(router, "/public/register", `{"username":"a","password":"b"}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"message":"validated!"}`, w.Body.String())

	w = postJSON(router, "/public/register", `{"username":"a"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error")
}

// ---------------------------------------------------------------------------
// t19: LoggingMiddleware keeps the request body readable

type captureHook struct {
	mu      sync.Mutex
	entries []capturedEntry
}

type capturedEntry struct {
	msg  string
	data log.Fields
}

func (h *captureHook) Levels() []log.Level { return log.AllLevels }

func (h *captureHook) Fire(e *log.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	data := log.Fields{}
	for k, v := range e.Data {
		data[k] = v
	}
	h.entries = append(h.entries, capturedEntry{msg: e.Message, data: data})
	return nil
}

func (h *captureHook) requests() []capturedEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []capturedEntry
	for _, e := range h.entries {
		if e.msg == "HTTP REQUEST" {
			out = append(out, e)
		}
	}
	return out
}

func captureLogs(t *testing.T) *captureHook {
	t.Helper()
	h := &captureHook{}
	old := log.StandardLogger().ReplaceHooks(make(log.LevelHooks))
	log.AddHook(h)
	t.Cleanup(func() { log.StandardLogger().ReplaceHooks(old) })
	return h
}

func TestIntegrationLoggingMiddlewareKeepsBody(t *testing.T) {
	hook := captureLogs(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(LoggingMiddleware())
	router.POST("/echo", func(c *gin.Context) {
		raw, err := c.GetRawData()
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			return
		}
		c.Data(http.StatusOK, "application/json", raw)
	})
	router.POST("/bind", func(c *gin.Context) {
		var p DTOPerson
		if err := c.BindJSON(&p); err != nil {
			return
		}
		c.JSON(http.StatusOK, gin.H{"firstname": p.FirstName, "uuid": p.UUID.String()})
	})

	body := `{"uuid":"` + itPersonUUID + `","firstname":"Jörg","lastname":"O'Neill"}`

	t.Run("raw body", func(t *testing.T) {
		w := postJSON(router, "/echo?x=1", body)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, body, w.Body.String())
	})

	t.Run("json binding", func(t *testing.T) {
		w := postJSON(router, "/bind", body)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"firstname":"Jörg","uuid":"`+itPersonUUID+`"}`, w.Body.String())
	})

	t.Run("empty body", func(t *testing.T) {
		w := postJSON(router, "/echo", "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Body.String())
	})

	entries := hook.requests()
	require.Len(t, entries, 3)
	assert.Equal(t, body, entries[0].data["6_BODY"])
	assert.Equal(t, http.MethodPost, entries[0].data["5_METHOD"])
	assert.Equal(t, "/echo?x=1", entries[0].data["2_URI"])
	assert.Equal(t, body, entries[1].data["6_BODY"])
	assert.Equal(t, "", entries[2].data["6_BODY"])
}

func TestIntegrationLoggingMiddlewareBodyThroughAPI(t *testing.T) {
	hook := captureLogs(t)
	newDB(t).
		on("select count(*) from person", rowOf("1")).
		on("SELECT id FROM adresse WHERE uuid", rowOf("7")).
		on("UPDATE person set", affected(1))

	body := mustJSON(t, DTOPerson{UUID: uuidMust(itPersonUUID), FirstName: "Max", LastName: "Muster", Sex: "male", AddressUUID: uuidMust(itAddressUUID)})
	w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, body)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := decodeBody[DTOPerson](t, w)
	assert.Equal(t, "Max", got.FirstName)

	entries := hook.requests()
	require.Len(t, entries, 1)
	assert.Equal(t, body, entries[0].data["6_BODY"])
	assert.Equal(t, http.MethodPut, entries[0].data["5_METHOD"])
}
