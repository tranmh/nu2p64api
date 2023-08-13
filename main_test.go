package main

// How to Write and Test a Rest API (CRUD) with GORM and Gin
// https://blog.hackajob.co/writing-and-testing-crud/

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
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
