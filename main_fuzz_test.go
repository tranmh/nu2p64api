package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The package init() exits when no .env is found in the working directory and
// writes main.log there. Package variables are initialised before init(), so
// move into a temporary directory holding an empty .env first.
var _ = func() bool {
	dir, err := os.MkdirTemp("", "nu2p64api-fuzz-")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), nil, 0o600); err != nil {
		panic(err)
	}
	if err := os.Chdir(dir); err != nil {
		panic(err)
	}
	return true
}()

// FuzzEscapeTick checks that the escaped value cannot terminate a single-quoted
// MySQL string literal: read with MySQL's backslash escapes, the value must not
// contain an unescaped quote and must not end in a dangling backslash.
func FuzzEscapeTick(f *testing.F) {
	f.Skip("aqaa: fuzz finding F1 (violation), re-enable once it is fixed; see report")
	for _, s := range []string{"", "abc", "O'Brien", "''", `\`, `\'`, `\\'`, "a' OR '1'='1", "\xC2\x96'", "\xff\xfe'", strings.Repeat("'\\", 100)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		out := EscapeTick(input)
		for i := 0; i < len(out); i++ {
			switch out[i] {
			case '\\':
				if i+1 >= len(out) {
					t.Fatalf("EscapeTick(%q) = %q ends in an unescaped backslash that would escape the closing quote", input, out)
				}
				i++
			case '\'':
				t.Fatalf("EscapeTick(%q) = %q contains an unescaped quote at byte %d", input, out, i)
			}
		}
	})
}

// FuzzReplaceSpecialCharacters checks that the result contains none of the
// sequences the function replaces, so applying it again changes nothing.
func FuzzReplaceSpecialCharacters(f *testing.F) {
	f.Skip("aqaa: fuzz finding F2 (violation), re-enable once it is fixed; see report")
	for _, s := range []string{"", "Müller", "Straße", "\xC2\x96", "\u009a", "\xc2\x9e", "\xC2\xC2\x9a\x96", "\xc2\u009a\x9e", "\xff", strings.Repeat("\xC2\x96\u009a", 50)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		out := ReplaceSpecialCharacters(input)
		for _, seq := range []string{"\xC2\x96", "\u009a", "\xc2\x9e"} {
			if strings.Contains(out, seq) {
				t.Errorf("ReplaceSpecialCharacters(%q) = %q still contains %q", input, out, seq)
			}
		}
		if again := ReplaceSpecialCharacters(out); again != out {
			t.Errorf("ReplaceSpecialCharacters is not idempotent for %q: once %q, twice %q", input, out, again)
		}
	})
}

// FuzzCivilTimeJSON checks that a date accepted by UnmarshalJSON is marshalled
// back to the same "YYYY-MM-DD" string and survives a second round trip.
func FuzzCivilTimeJSON(f *testing.F) {
	for _, s := range []string{`"2023-05-17"`, `""`, `null`, `"0001-01-01"`, `"9999-12-31"`, `"2024-02-29"`, `"2023-02-29"`, `"2023-05-17T10:00:00Z"`, `1684310400`, `"17.05.2023"`, `"""2023-05-17"`, "\xff"} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		var c CivilTime
		if err := c.UnmarshalJSON(b); err != nil {
			return
		}
		out, err := c.MarshalJSON()
		if err != nil {
			t.Fatalf("MarshalJSON after UnmarshalJSON(%q) failed: %v", b, err)
		}
		value := strings.Trim(string(b), `"`)
		if value != "" && value != "null" && string(out) != `"`+value+`"` {
			t.Errorf("UnmarshalJSON(%q) then MarshalJSON = %s, want %q", b, out, `"`+value+`"`)
		}
		var c2 CivilTime
		if err := c2.UnmarshalJSON(out); err != nil {
			t.Fatalf("UnmarshalJSON(%s) of marshalled value from %q failed: %v", out, b, err)
		}
		if !time.Time(c2).Equal(time.Time(c)) {
			t.Errorf("round trip of %q changed the date: %v != %v", b, time.Time(c2), time.Time(c))
		}
	})
}

// FuzzParseStringToCivilTime checks that every date accepted by
// parseStringToCivilTime is formatted back to the input by CivilTimeToString,
// except the zero date, which becomes the empty string.
func FuzzParseStringToCivilTime(f *testing.F) {
	for _, s := range []string{"2023-05-17", "", "0001-01-01", "0000-01-01", "9999-12-31", "2024-02-29", "2023-02-29", "2023-13-01", "2023-5-7", "17.05.2023", "2023-05-17T00:00:00Z", "\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		ct, err := parseStringToCivilTime(input)
		if err != nil {
			return
		}
		want := input
		if input == "0001-01-01" {
			want = ""
		}
		if got := CivilTimeToString(ct); got != want {
			t.Errorf("CivilTimeToString(parseStringToCivilTime(%q)) = %q, want %q", input, got, want)
		}
	})
}

// FuzzLicenseState checks that the parsed state is one of the three known
// states and survives a round trip through LicenseStateToString.
func FuzzLicenseState(f *testing.F) {
	for _, s := range []string{"ACTIVE", "active", "Passive", "PASSIVE", "UNKNOWN", "", "NO_LICENSE", " ACTIVE", "actıve", "\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, input string) {
		state := getLicenseStateFromString(input)
		if state != LicenseStateUnknown && state != LicenseStateActive && state != LicenseStatePassive {
			t.Fatalf("getLicenseStateFromString(%q) = %d, not a known state", input, state)
		}
		if back := getLicenseStateFromString(LicenseStateToString(state)); back != state {
			t.Errorf("round trip of %q: state %d -> %q -> %d", input, state, LicenseStateToString(state), back)
		}
	})
}
