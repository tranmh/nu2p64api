package main

// How to Write and Test a Rest API (CRUD) with GORM and Gin
// https://blog.hackajob.co/writing-and-testing-crud/

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_GetPersonTranByUUID(t *testing.T) {
	db, err := sql.Open("mysql", "portal:Usm@1?/#Qv^avF@tcp(127.0.0.1:3306)/mvdsb")
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	uuidParam := "babe8313-f269-11ed-927b-005056054f4e"

	var person DTOPerson

	if isValidUUID(uuidParam) {
		myUuid, _ := uuid.Parse(uuidParam)
		person.UUID = myUuid
		var birthdate []uint8

		err := db.QueryRow("SELECT vorname, name, geschlecht, geburtsdatum FROM `person` where uuid = ?", myUuid).
			Scan(
				&person.FirstName,
				&person.LastName,
				&person.Gender,
				&birthdate,
				// &person.birthdate,
				// &person.BirthPlace,
			)
		if err != nil {
			t.Error(err.Error())
		} else {
			p, err := json.Marshal(person)
			if err != nil {
				t.Error(err)
			}
			fmt.Println(string(p))
			fmt.Println(person)
		}
	} else {
		t.Error("isValidUUID(uuidParam) is false")
	}
}

// Göppingen: https://anybody:s3cr3t@test.svw.info:3030/api/federations/6e358ea2-f26a-11ed-927b-005056054f4e
// Cuong: https://anybody:s3cr3t@test.svw.info:3030/api/persons/babe8313-f269-11ed-927b-005056054f4e
// Deutscher Schachbund: https://anybody:s3cr3t@test.svw.info:3030/api/federations/6e25f2a5-f26a-11ed-927b-005056054f4e

func Test_loginUser(t *testing.T) {

	token, err := loginCheck("anybody", "s3cr3t")

	if err != nil {
		t.Error("err was not nil")
	}
	fmt.Println(token)

}

func Test_validateDTOAddress(t *testing.T) {
	var address DTOAddress
	address.WWW = "www.schachvereine.de"

	result, _ := validateDTOAddress(address)

	fmt.Println(result)
	if result == false {
		t.Error("result should be true")
	}

}

func Test_EscapeTick(t *testing.T) {
	fmt.Println(EscapeTick("O'Connor"))
}

func Test_UTF8(t *testing.T) {
	fmt.Println(ReplaceSpecialCharacters("Lange Ã\u0084cker 14"))
}

func Test_PrintTime(t *testing.T) {
	fmt.Println(time.Now().Format(time.RFC3339))
}

func Test_verifyPassword(t *testing.T) {
	assert.Equal(t, false, verifyPassword("dead", "beef"))
}

func Test_ReplaceSpecialCharacters(t *testing.T) {
	assert.Equal(t, "Lange cker 14", ReplaceSpecialCharacters("Lange \u0084cker 14"))
}

func Test_ClubTypeStringToistAbteilung(t *testing.T) {
	assert.Equal(t, "0", ClubTypeStringToistAbteilung("SINGLEDEVISION"))
	assert.Equal(t, "1", ClubTypeStringToistAbteilung("MULTIDIVISION"))
	assert.Equal(t, "2", ClubTypeStringToistAbteilung("X"))
}

func TestCivilTime(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name           string
		input          string
		expectedOutput string
		expectError    bool
	}{
		{
			name:           "Valid Date",
			input:          `"2022-01-01"`,
			expectedOutput: `"2022-01-01"`,
			expectError:    false,
		},
		{
			name:           "Invalid Date",
			input:          `"invalid-date"`,
			expectedOutput: "",
			expectError:    true,
		},
		{
			name:           "Empty Date",
			input:          `""`,
			expectedOutput: `"0001-01-01"`,
			expectError:    false,
		},
		{
			name:           "Null Date",
			input:          `"null"`,
			expectedOutput: `"0001-01-01"`,
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a CivilTime object
			var ct CivilTime

			// Unmarshal the input into the CivilTime object
			err := ct.UnmarshalJSON([]byte(tc.input))

			// Check if an error was returned
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Marshal the CivilTime object back into a string
				output, err := ct.MarshalJSON()
				assert.NoError(t, err)

				// Check if the output matches the expected output
				assert.Equal(t, tc.expectedOutput, string(output))
			}
		})
	}
}
