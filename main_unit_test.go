package main

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func civil(y int, m time.Month, d int) CivilTime {
	return CivilTime(time.Date(y, m, d, 0, 0, 0, 0, time.UTC))
}

// ---------------------------------------------------------------------------
// t1: CivilTime JSON (B8, B9) and parseStringToCivilTime / CivilTimeToString (B10)

func TestCivilTimeUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    CivilTime
		wantErr bool
	}{
		{name: "valid date", input: `"2022-01-01"`, want: civil(2022, 1, 1)},
		{name: "leap day", input: `"2024-02-29"`, want: civil(2024, 2, 29)},
		{name: "last day of year", input: `"1999-12-31"`, want: civil(1999, 12, 31)},
		{name: "zero date", input: `"0001-01-01"`, want: CivilTime{}},
		{name: "unquoted value is trimmed too", input: `2022-03-04`, want: civil(2022, 3, 4)},
		{name: "impossible date without separator", input: `"2022-0230"`, wantErr: true},
		{name: "february 30", input: `"2022-02-30"`, wantErr: true},
		{name: "february 29 in non leap year", input: `"2023-02-29"`, wantErr: true},
		{name: "month 13", input: `"2022-13-01"`, wantErr: true},
		{name: "month 00", input: `"2022-00-10"`, wantErr: true},
		{name: "day 00", input: `"2022-01-00"`, wantErr: true},
		{name: "german format", input: `"01.01.2022"`, wantErr: true},
		{name: "rfc3339 timestamp", input: `"2022-01-01T00:00:00Z"`, wantErr: true},
		{name: "single digit month", input: `"2022-1-01"`, wantErr: true},
		{name: "garbage", input: `"invalid-date"`, wantErr: true},
		{name: "whitespace only", input: `" "`, wantErr: true},
		{name: "number", input: `20220101`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ct CivilTime
			err := ct.UnmarshalJSON([]byte(tc.input))
			if tc.wantErr {
				assert.Error(t, err)
				assert.True(t, time.Time(ct).IsZero(), "value must stay untouched on error")
				return
			}
			require.NoError(t, err)
			assert.True(t, time.Time(tc.want).Equal(time.Time(ct)), "got %v want %v", time.Time(ct), time.Time(tc.want))
		})
	}
}

func TestCivilTimeUnmarshalJSONEmptyAndNullKeepValue(t *testing.T) {
	prev := civil(2020, 5, 17)
	for _, input := range []string{`""`, `null`, `"null"`} {
		t.Run(input, func(t *testing.T) {
			ct := prev
			err := ct.UnmarshalJSON([]byte(input))
			require.NoError(t, err)
			assert.True(t, time.Time(prev).Equal(time.Time(ct)), "value must stay unchanged, got %v", time.Time(ct))
		})
	}
}

func TestCivilTimeUnmarshalViaEncodingJSON(t *testing.T) {
	type wrapper struct {
		D CivilTime `json:"d"`
	}

	t.Run("valid", func(t *testing.T) {
		var w wrapper
		require.NoError(t, json.Unmarshal([]byte(`{"d":"2023-07-28"}`), &w))
		assert.Equal(t, "2023-07-28", time.Time(w.D).Format("2006-01-02"))
	})

	t.Run("json null leaves value unchanged", func(t *testing.T) {
		w := wrapper{D: civil(2020, 1, 2)}
		require.NoError(t, json.Unmarshal([]byte(`{"d":null}`), &w))
		assert.Equal(t, "2020-01-02", time.Time(w.D).Format("2006-01-02"))
	})

	t.Run("invalid date is an error", func(t *testing.T) {
		var w wrapper
		assert.Error(t, json.Unmarshal([]byte(`{"d":"2022-0230"}`), &w))
	})

	t.Run("dto with dates", func(t *testing.T) {
		var m DTOClubMember
		body := `{"member-from":"2001-02-03","member-until":"","license-valid-from":null}`
		require.NoError(t, json.Unmarshal([]byte(body), &m))
		assert.Equal(t, "2001-02-03", CivilTimeToString(m.Member_From))
		assert.Equal(t, "", CivilTimeToString(m.Member_Until))
		assert.Equal(t, "", CivilTimeToString(m.License_Valid_From))
	})
}

func TestCivilTimeMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   CivilTime
		want string
	}{
		{name: "zero time", in: CivilTime{}, want: `"0001-01-01"`},
		{name: "regular date", in: civil(2022, 1, 1), want: `"2022-01-01"`},
		{name: "time of day is dropped", in: CivilTime(time.Date(2022, 6, 15, 23, 59, 59, 999, time.UTC)), want: `"2022-06-15"`},
		{name: "leap day", in: civil(2024, 2, 29), want: `"2024-02-29"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.in.MarshalJSON()
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(out))
		})
	}
}

func TestCivilTimeJSONRoundTripInDTO(t *testing.T) {
	in := DTOPlayerLicense{
		UUID:             uuid.MustParse("babe8313-f269-11ed-927b-005056054f4e"),
		RequestDate:      civil(2023, 4, 22),
		LicenseValidFrom: civil(2023, 5, 1),
		License_State:    "ACTIVE",
	}
	b, err := json.Marshal(in)
	require.NoError(t, err)
	s := string(b)
	assert.Contains(t, s, `"request-date":"2023-04-22"`)
	assert.Contains(t, s, `"licence-valid-from":"2023-05-01"`)
	assert.Contains(t, s, `"licence-valid-until":"0001-01-01"`)

	var out DTOPlayerLicense
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, CivilTimeToString(in.RequestDate), CivilTimeToString(out.RequestDate))
	assert.Equal(t, CivilTimeToString(in.LicenseValidFrom), CivilTimeToString(out.LicenseValidFrom))
	assert.Equal(t, in.UUID, out.UUID)
}

func TestParseStringToCivilTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "valid", input: "1980-07-15", want: "1980-07-15"},
		{name: "leap day", input: "2000-02-29", want: "2000-02-29"},
		{name: "zero date", input: "0001-01-01", want: ""},
		{name: "empty", input: "", wantErr: true},
		{name: "impossible date", input: "2022-0230", wantErr: true},
		{name: "february 30", input: "2022-02-30", wantErr: true},
		{name: "mysql zero date", input: "0000-00-00", wantErr: true},
		{name: "datetime", input: "1980-07-15 00:00:00", wantErr: true},
		{name: "german format", input: "15.07.1980", wantErr: true},
		{name: "quoted", input: `"1980-07-15"`, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseStringToCivilTime(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, CivilTimeToString(got))
		})
	}
}

func TestCivilTimeToString(t *testing.T) {
	tests := []struct {
		name string
		in   CivilTime
		want string
	}{
		{name: "zero date renders empty", in: CivilTime{}, want: ""},
		{name: "regular", in: civil(2023, 7, 28), want: "2023-07-28"},
		{name: "padding", in: civil(5, 1, 2), want: "0005-01-02"},
		{name: "day after zero date", in: civil(1, 1, 2), want: "0001-01-02"},
		{name: "zero date with time of day still renders empty", in: CivilTime(time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC)), want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, CivilTimeToString(tc.in))
		})
	}
}

// ---------------------------------------------------------------------------
// t2: validateDTOAddress (B16)

func TestValidateDTOAddress(t *testing.T) {
	tests := []struct {
		name      string
		addr      DTOAddress
		wantOK    bool
		errSubstr string
	}{
		{name: "all blank", addr: DTOAddress{}, wantOK: true},
		{name: "valid email", addr: DTOAddress{Email: "max@example.com"}, wantOK: true},
		{name: "valid email with display name", addr: DTOAddress{Email: "Max Muster <max@example.com>"}, wantOK: true},
		{name: "valid email2", addr: DTOAddress{Email2: "info@schach.de"}, wantOK: true},
		{name: "whitespace only email", addr: DTOAddress{Email: "   "}, wantOK: true},
		{name: "whitespace only email2", addr: DTOAddress{Email2: "\t "}, wantOK: true},
		{name: "invalid email no at", addr: DTOAddress{Email: "not-an-email"}, wantOK: false},
		{name: "invalid email no domain", addr: DTOAddress{Email: "max@"}, wantOK: false},
		{name: "invalid email no local part", addr: DTOAddress{Email: "@example.com"}, wantOK: false},
		{name: "invalid email two addresses", addr: DTOAddress{Email: "a@b.de, c@d.de"}, wantOK: false},
		{name: "invalid email2", addr: DTOAddress{Email: "ok@example.com", Email2: "broken"}, wantOK: false},
		{name: "bare domain www gets https prefix", addr: DTOAddress{WWW: "www.schachvereine.de"}, wantOK: true},
		{name: "bare domain without www", addr: DTOAddress{WWW: "svw.info"}, wantOK: true},
		{name: "http url", addr: DTOAddress{WWW: "http://svw.info/verein"}, wantOK: true},
		{name: "https url upper case scheme", addr: DTOAddress{WWW: "HTTPS://SVW.INFO"}, wantOK: true},
		{name: "whitespace only www", addr: DTOAddress{WWW: "   "}, wantOK: true},
		{name: "www with space in host", addr: DTOAddress{WWW: "www.schach verein.de"}, wantOK: false},
		{name: "www with broken ipv6 host", addr: DTOAddress{WWW: "http://[::1"}, wantOK: false},
		{name: "phone international plus", addr: DTOAddress{Phone_Mobile: "+49 171 1234567"}, wantOK: true},
		{name: "phone international 00", addr: DTOAddress{Phone_Home: "0049 711 123456"}, wantOK: true},
		{name: "phone with slash", addr: DTOAddress{Phone_Work: "0711/123456"}, wantOK: true},
		{name: "phone with brackets and dash", addr: DTOAddress{Phone_Home: "(0711) 123-456"}, wantOK: true},
		{name: "phone with ext.", addr: DTOAddress{Phone_Work: "0711 123456 ext. 12"}, wantOK: true},
		{name: "phone with x extension", addr: DTOAddress{Phone_Work: "0711 123456 x12"}, wantOK: true},
		{name: "phone with hash extension", addr: DTOAddress{Phone_Work: "0711 123456 #12"}, wantOK: true},
		{name: "phone mobile letters", addr: DTOAddress{Phone_Mobile: "abc"}, wantOK: false, errSubstr: "Phone_Mobile"},
		{name: "phone home vanity", addr: DTOAddress{Phone_Home: "0711-CALL-ME"}, wantOK: false, errSubstr: "Phone_Home"},
		{name: "phone work injection", addr: DTOAddress{Phone_Work: "0711'; DROP TABLE person; --"}, wantOK: false, errSubstr: "Phone_Work"},
		{
			name: "full valid address",
			addr: DTOAddress{
				Name: "SV Musterstadt", Street: "Hauptstr. 1", ZIP: "70173", City: "Stuttgart", Country: "DEU",
				Email: "info@sv.de", Email2: "vorstand@sv.de", WWW: "sv.de",
				Phone_Mobile: "+49 171 1234567", Phone_Home: "0711 123456", Phone_Work: "0711-654321",
			},
			wantOK: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := validateDTOAddress(tc.addr)
			assert.Equal(t, tc.wantOK, ok)
			if tc.wantOK {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			if tc.errSubstr != "" {
				assert.Contains(t, err.Error(), tc.errSubstr)
			}
		})
	}
}

func TestValidateDTOAddressDoesNotMutateInput(t *testing.T) {
	addr := DTOAddress{WWW: "www.schachvereine.de"}
	ok, err := validateDTOAddress(addr)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "www.schachvereine.de", addr.WWW)
}

// ---------------------------------------------------------------------------
// t3: EscapeTick / ReplaceSpecialCharacters (B15)

func TestReplaceSpecialCharacters(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain ascii unchanged", in: "Hauptstrasse 1", want: "Hauptstrasse 1"},
		{name: "umlauts unchanged", in: "Göppingen Straße", want: "Göppingen Straße"},
		{name: "empty", in: "", want: ""},
		{name: "C2 96 becomes space", in: "Baden\xC2\x96Württemberg", want: "Baden Württemberg"},
		{name: "U+009A removed", in: "a\u009ab", want: "ab"},
		{name: "C2 9E becomes s", in: "Stra\xC2\x9Ee", want: "Strase"},
		{name: "U+009E is the same sequence", in: "Stra\u009ee", want: "Strase"},
		{name: "multiple occurrences", in: "\xC2\x96x\xC2\x96\u009a\xC2\x9E\xC2\x9E", want: " x ss"},
		{name: "lone continuation byte untouched", in: "a\x96b", want: "a\x96b"},
		{name: "quotes untouched", in: "O'Connor", want: "O'Connor"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ReplaceSpecialCharacters(tc.in))
		})
	}
}

func TestEscapeTick(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "no quote", in: "Mueller", want: "Mueller"},
		{name: "empty", in: "", want: ""},
		{name: "single quote", in: "O'Connor", want: `O\'Connor`},
		{name: "two quotes", in: "''", want: `\'\'`},
		{name: "leading and trailing quote", in: "'x'", want: `\'x\'`},
		{name: "special characters replaced first", in: "D'\xC2\x96Stra\xC2\x9Ee\u009a", want: `D\' Strase`},
		{name: "double quote untouched", in: `say "hi"`, want: `say "hi"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, EscapeTick(tc.in))
		})
	}
}

// mysqlLiteralEnd scans s, which is the content following an opening single
// quote of a MySQL string literal (default sql_mode, backslash escapes active),
// and returns the index of the closing quote, or -1 if the literal never ends.
func mysqlLiteralEnd(s string) int {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++ // escaped character
		case '\'':
			if i+1 < len(s) && s[i+1] == '\'' {
				i++ // '' is an escaped quote
				continue
			}
			return i
		}
	}
	return -1
}

func assertEscapedStaysInsideLiteral(t *testing.T, input string) {
	t.Helper()
	// queries are built as: ... = '<EscapeTick(input)>'
	lit := EscapeTick(input) + "'"
	assert.Equal(t, len(lit)-1, mysqlLiteralEnd(lit),
		"escaped value %q must not terminate the surrounding quoted literal early or swallow its closing quote", EscapeTick(input))
}

func TestEscapeTickKeepsValueInsideLiteral(t *testing.T) {
	for _, in := range []string{"O'Connor", "' OR '1'='1", "x'; DROP TABLE person; --", "''''", "plain"} {
		t.Run(in, func(t *testing.T) {
			assertEscapedStaysInsideLiteral(t, in)
		})
	}
}

// aqaa: suspected bug — EscapeTick only escapes single quotes. MySQL treats a
// backslash as an escape character, so a value containing a backslash breaks out
// of the quoted literal in all concatenated queries (SQL injection): a trailing
// backslash swallows the closing quote, and `\'` becomes `\\'` which closes the
// literal. B15 documents the quote-only escaping, but it is not safe for MySQL.
func TestEscapeTickBackslashInjection(t *testing.T) {
	for _, in := range []string{`abc\`, `\' OR 1=1 -- `, `C:\temp\`} {
		t.Run(in, func(t *testing.T) {
			assertEscapedStaysInsideLiteral(t, in)
		})
	}
}

// ---------------------------------------------------------------------------
// t4: extractToken (B4)

func TestExtractToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		target string
		header string // Authorization header, "" = not set
		want   string
	}{
		{name: "no query no header", target: "/api/persons", want: ""},
		{name: "bearer header", target: "/api/persons", header: "Bearer abc.def.ghi", want: "abc.def.ghi"},
		{name: "query token", target: "/api/persons?token=q123", want: "q123"},
		{name: "query token takes precedence over header", target: "/api/persons?token=q123", header: "Bearer h456", want: "q123"},
		{name: "empty query token falls back to header", target: "/api/persons?token=", header: "Bearer h456", want: "h456"},
		{name: "single word header", target: "/api/persons", header: "Bearer", want: ""},
		{name: "single token without scheme", target: "/api/persons", header: "abc.def.ghi", want: ""},
		{name: "three parts", target: "/api/persons", header: "Bearer a b", want: ""},
		{name: "double space", target: "/api/persons", header: "Bearer  abc", want: ""},
		{name: "two parts with any scheme", target: "/api/persons", header: "Basic abc", want: "abc"},
		{name: "trailing space gives empty token", target: "/api/persons", header: "Bearer ", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, tc.target, nil)
			if tc.header != "" {
				c.Request.Header.Set("Authorization", tc.header)
			}
			assert.Equal(t, tc.want, extractToken(c))
		})
	}
}

// ---------------------------------------------------------------------------
// t7: enum and string mappers (B11, B12, B13, B14)

func TestGetSex(t *testing.T) {
	tests := []struct {
		in   string
		want Sex
	}{
		{"female", Female},
		{"male", Male},
		{"", SexUnknown},
		{"nonbinary", SexUnknown},
		{"Male", SexUnknown},
		{"FEMALE", SexUnknown},
		{" male", SexUnknown},
		{"0", SexUnknown},
		{"1", SexUnknown},
		{"sexunknown", SexUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, getSex(tc.in))
		})
	}
}

func TestSexString(t *testing.T) {
	tests := []struct {
		in   Sex
		want string
	}{
		{Male, "male"},
		{Female, "female"},
		{SexUnknown, "sexunknown"},
		{Sex(-1), "sexunknown"},
		{Sex(42), "sexunknown"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.in.String())
		})
	}
}

func TestSexRoundTripAndDBEncoding(t *testing.T) {
	for _, s := range []Sex{Female, Male, SexUnknown} {
		assert.Equal(t, s, getSex(s.String()))
	}
	// person.geschlecht in the database: 0 = female, 1 = male (B17)
	assert.Equal(t, 0, int(Female))
	assert.Equal(t, 1, int(Male))
	assert.Equal(t, 2, int(SexUnknown))
}

func TestGetLicenseStateFromString(t *testing.T) {
	tests := []struct {
		in   string
		want LicenseState
	}{
		{"ACTIVE", LicenseStateActive},
		{"active", LicenseStateActive},
		{"Active", LicenseStateActive},
		{"PASSIVE", LicenseStatePassive},
		{"passive", LicenseStatePassive},
		{"PaSsIvE", LicenseStatePassive},
		{"", LicenseStateUnknown},
		{"UNKNOWN", LicenseStateUnknown},
		{"NO_LICENSE", LicenseStateUnknown},
		{" ACTIVE", LicenseStateUnknown},
		{"1", LicenseStateUnknown},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, getLicenseStateFromString(tc.in))
		})
	}
}

func TestLicenseStateToString(t *testing.T) {
	tests := []struct {
		in   LicenseState
		want string
	}{
		{LicenseStateActive, "ACTIVE"},
		{LicenseStatePassive, "PASSIVE"},
		{LicenseStateUnknown, "UNKNOWN"},
		{LicenseState(3), "UNKNOWN"},
		{LicenseState(-1), "UNKNOWN"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, LicenseStateToString(tc.in))
		})
	}
	for _, s := range []string{"ACTIVE", "PASSIVE", "UNKNOWN"} {
		assert.Equal(t, s, LicenseStateToString(getLicenseStateFromString(s)))
	}
}

func TestIstAbteilungToClubType(t *testing.T) {
	tests := []struct {
		in   istAbteilung
		want string
	}{
		{Einsparten, "SINGLEDEVISION"},
		{Mehrsparten, "MULTIDIVISION"},
		{UNKNOWN_istAbteilung, "UNKNOWN_CLUBTYPE"},
		{istAbteilung(-1), "UNKNOWN_CLUBTYPE"},
		{istAbteilung(7), "UNKNOWN_CLUBTYPE"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, istAbteilungToClubType(tc.in))
		})
	}
}

func TestClubTypeStringToistAbteilungMapping(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"SINGLEDEVISION", "0"},
		{"singledevision", "0"},
		{"SingleDevision", "0"},
		{"MULTIDIVISION", "1"},
		{"multidivision", "1"},
		{"", "2"},
		{"UNKNOWN_CLUBTYPE", "2"},
		{"SINGLEDIVISION", "2"}, // correct English spelling is not accepted, the API uses "DEVISION"
		{" MULTIDIVISION", "2"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, ClubTypeStringToistAbteilung(tc.in))
		})
	}
	// round trip through both mappers
	for _, ia := range []istAbteilung{Einsparten, Mehrsparten} {
		assert.Equal(t, ia.stringValue(), ClubTypeStringToistAbteilung(istAbteilungToClubType(ia)))
	}
	assert.Equal(t, "2", ClubTypeStringToistAbteilung(istAbteilungToClubType(UNKNOWN_istAbteilung)))
}

func (ia istAbteilung) stringValue() string {
	switch ia {
	case Einsparten:
		return "0"
	case Mehrsparten:
		return "1"
	}
	return "2"
}

func TestConvertTitleToTitleID(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 1},
		{"Dr.", 2},
		{"Prof.", 3},
		{"Prof. Dr.", 4},
		{"Dr", 5},
		{"dr.", 5},
		{" Dr.", 5},
		{"Dr. Prof.", 5},
		{"Prof.Dr.", 5},
		{"Dipl.-Ing.", 5},
		{" ", 5},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.want, convertTitleToTitleID(tc.in))
		})
	}
}

// ---------------------------------------------------------------------------
// t20: getSportAddressUUIDs with a fake database

func TestGetSportAddressUUIDs(t *testing.T) {
	u1 := "6e358ea2-f26a-11ed-927b-005056054f4e"
	u2 := "babe8313-f269-11ed-927b-005056054f4e"

	t.Run("empty result", func(t *testing.T) {
		conn := useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{cols: []string{"uuid"}}
		})
		got, err := getSportAddressUUIDs(42)
		require.NoError(t, err)
		assert.Empty(t, got)
		require.Len(t, conn.Queries(), 1)
		q := conn.Queries()[0]
		assert.Contains(t, q, "FROM adressen")
		assert.Contains(t, q, "typ = 5")
		assert.True(t, strings.HasSuffix(q, "organisation = 42"), q)
	})

	t.Run("multiple rows", func(t *testing.T) {
		useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{cols: []string{"uuid"}, rows: [][]driver.Value{{u1}, {[]byte(u2)}}}
		})
		got, err := getSportAddressUUIDs(7)
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{uuid.MustParse(u1), uuid.MustParse(u2)}, got)
	})

	t.Run("invalid uuid in a row", func(t *testing.T) {
		useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{cols: []string{"uuid"}, rows: [][]driver.Value{{u1}, {"not-a-uuid"}}}
		})
		got, err := getSportAddressUUIDs(7)
		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("empty uuid string in a row", func(t *testing.T) {
		useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{cols: []string{"uuid"}, rows: [][]driver.Value{{""}}}
		})
		got, err := getSportAddressUUIDs(7)
		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("NULL uuid fails to scan", func(t *testing.T) {
		useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{cols: []string{"uuid"}, rows: [][]driver.Value{{nil}}}
		})
		got, err := getSportAddressUUIDs(7)
		assert.Error(t, err)
		assert.Nil(t, got)
	})

	t.Run("query error", func(t *testing.T) {
		dbErr := errors.New("connection lost")
		useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{err: dbErr}
		})
		got, err := getSportAddressUUIDs(7)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, got)
	})

	t.Run("negative organisation id is rendered into the query", func(t *testing.T) {
		conn := useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{cols: []string{"uuid"}}
		})
		_, err := getSportAddressUUIDs(-1)
		require.NoError(t, err)
		assert.True(t, strings.HasSuffix(conn.Queries()[0], "organisation = -1"))
	})
}

// aqaa: suspected bug — getSportAddressUUIDs never checks rows.Err() after the
// loop. An error raised while iterating (e.g. a dropped connection after the
// first row) is swallowed and a truncated list is returned with a nil error.
func TestGetSportAddressUUIDsIterationError(t *testing.T) {
	t.Skip("aqaa: open bug, see report")
	iterErr := errors.New("connection reset while reading rows")
	useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
		return fakeResult{
			cols:    []string{"uuid"},
			rows:    [][]driver.Value{{"6e358ea2-f26a-11ed-927b-005056054f4e"}},
			rowsErr: iterErr,
		}
	})
	got, err := getSportAddressUUIDs(7)
	assert.ErrorIs(t, err, iterErr)
	assert.Nil(t, got)
}

// ---------------------------------------------------------------------------
// B22: ClubRoleNameToFunktion uses EscapeTick and falls back to "0"

func TestClubRoleNameToFunktion(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		conn := useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{cols: []string{"id"}, rows: [][]driver.Value{{"17"}}}
		})
		assert.Equal(t, "17", ClubRoleNameToFunktion("Spielleiter"))
		assert.Equal(t, `select id from funktionsart where bezeichnung = 'Spielleiter'`, conn.Queries()[0])
	})

	t.Run("quote is escaped in query", func(t *testing.T) {
		conn := useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{cols: []string{"id"}, rows: [][]driver.Value{{"3"}}}
		})
		assert.Equal(t, "3", ClubRoleNameToFunktion("Jugend'wart"))
		assert.Contains(t, conn.Queries()[0], `'Jugend\'wart'`)
	})

	t.Run("no rows", func(t *testing.T) {
		useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{cols: []string{"id"}}
		})
		assert.Equal(t, "0", ClubRoleNameToFunktion("unbekannt"))
	})

	t.Run("db error", func(t *testing.T) {
		useFakeDB(t, func(string, []driver.NamedValue) fakeResult {
			return fakeResult{err: errors.New("boom")}
		})
		assert.Equal(t, "0", ClubRoleNameToFunktion("Kassenwart"))
	})
}
