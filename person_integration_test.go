//go:build integration

package main

import (
	"database/sql/driver"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// t8: GET /api/persons/:pers_uuid (B17, B18)

const personMainQuery = "from person, titel"

// personRow is the row of the main person select in column order:
// name, vorname, titel, geschlecht, adress, geburtsdatum, geburtsort,
// verstorben, nation, datenschutz, bemerkung, nationfide, idfide.
func personRow(sex, privacy string) fakeResult {
	return rowOf("Mustermann", "Max", "Dr.", sex, "7", "1980-07-15", "Stuttgart", "0", "GER", privacy, "Bemerkung", "GER", "4611016")
}

func personDB(t *testing.T, row fakeResult) *dbScript {
	return newDB(t).
		on("select count(*) from person", rowOf("1")).
		on(personMainQuery, row).
		on("select uuid from adresse where id = 7", rowOf(itAddressUUID))
}

func TestIntegrationGetPersonSuccess(t *testing.T) {
	db := personDB(t, personRow("1", "0"))

	w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got := decodeBody[DTOPerson](t, w)
	assert.Equal(t, itPersonUUID, got.UUID.String())
	assert.Equal(t, "Dr.", got.Title)
	assert.Equal(t, "male", got.Sex)
	assert.Equal(t, itAddressUUID, got.AddressUUID.String())
	assert.Equal(t, "1980-07-15", CivilTimeToString(got.BirthDate))
	assert.Equal(t, "Stuttgart", got.BirthPlace)
	assert.Equal(t, 0, got.Dead)
	assert.Equal(t, "GER", got.Nation)
	assert.False(t, got.Privacy_State)
	assert.Equal(t, "Bemerkung", got.Remarks)
	assert.Equal(t, "GER", got.FIDE_Nation)
	assert.Equal(t, "4611016", got.FIDE_Id)
	assert.Contains(t, w.Body.String(), `"birthdate":"1980-07-15"`)

	count := db.one(t, "select count(*) from person")
	assert.Contains(t, count.query, "uuid = '"+itPersonUUID+"'")
	main := db.one(t, personMainQuery)
	assert.Equal(t, []driver.Value{itPersonUUID}, main.args)
}

func TestIntegrationGetPersonSexMapping(t *testing.T) {
	tests := []struct {
		sex        string
		wantStatus int
		wantSex    string
	}{
		{sex: "0", wantStatus: 200, wantSex: "female"},
		{sex: "1", wantStatus: 200, wantSex: "male"},
		{sex: "2", wantStatus: 500},
		{sex: "", wantStatus: 500},
		{sex: "male", wantStatus: 500},
	}
	for _, tc := range tests {
		t.Run("geschlecht="+tc.sex, func(t *testing.T) {
			db := personDB(t, personRow(tc.sex, "0"))
			w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
			require.Equal(t, tc.wantStatus, w.Code, w.Body.String())
			if tc.wantStatus == 200 {
				assert.Equal(t, tc.wantSex, decodeBody[DTOPerson](t, w).Sex)
			} else {
				assert.Empty(t, db.all("from adresse"), "must stop before the address lookup")
			}
		})
	}
}

func TestIntegrationGetPersonPrivacyStateValidation(t *testing.T) {
	tests := []struct {
		privacy    string
		wantStatus int
	}{
		{privacy: "0", wantStatus: 200},
		{privacy: "", wantStatus: 500},
		{privacy: "2", wantStatus: 500},
		{privacy: "true", wantStatus: 500},
	}
	for _, tc := range tests {
		t.Run("datenschutz="+tc.privacy, func(t *testing.T) {
			personDB(t, personRow("0", tc.privacy))
			w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
			require.Equal(t, tc.wantStatus, w.Code, w.Body.String())
			if tc.wantStatus == 200 {
				assert.False(t, decodeBody[DTOPerson](t, w).Privacy_State)
			}
		})
	}
}

// aqaa: suspected bug (B18) — datenschutz = 1 means "zugestimmt" (consent given),
// but getDTOPerson maps both "0" and "1" to Privacy_State=false.
func TestIntegrationGetPersonPrivacyStateConsent(t *testing.T) {
	t.Skip("aqaa: open bug, see report")
	personDB(t, personRow("0", "1"))
	w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.True(t, decodeBody[DTOPerson](t, w).Privacy_State, "datenschutz=1 (zugestimmt) must map to privacy-state=true")
}

// aqaa: suspected bug — the select returns person.name (family name) first and
// person.vorname second, but they are scanned into FirstName and LastName in
// that order, so firstname and lastname are swapped in the response.
// putDTOPerson writes name=LastName and vorname=FirstName, so GET does not
// round-trip what PUT stored.
func TestIntegrationGetPersonNameMapping(t *testing.T) {
	t.Skip("aqaa: open bug, see report")
	personDB(t, personRow("1", "0")) // name = Mustermann, vorname = Max
	w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got := decodeBody[DTOPerson](t, w)
	assert.Equal(t, "Max", got.FirstName)
	assert.Equal(t, "Mustermann", got.LastName)
}

func TestIntegrationGetPersonErrors(t *testing.T) {
	t.Run("invalid uuid", func(t *testing.T) {
		db := newDB(t)
		w := callAPI(http.MethodGet, "/api/persons/not-a-uuid", "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "uuidParam is not valid: not-a-uuid")
		assert.Empty(t, db.queries())
	})
	t.Run("not found", func(t *testing.T) {
		db := newDB(t).on("select count(*) from person", rowOf("0"))
		w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Contains(t, w.Body.String(), itPersonUUID)
		assert.Empty(t, db.all(personMainQuery))
	})
	t.Run("count query fails", func(t *testing.T) {
		newDB(t).on("select count(*) from person", dbFail("connection refused"))
		w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "connection refused")
	})
	t.Run("main select fails", func(t *testing.T) {
		newDB(t).
			on("select count(*) from person", rowOf("1")).
			on(personMainQuery, dbFail("lost connection"))
		w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
	t.Run("invalid birth date", func(t *testing.T) {
		personDB(t, rowOf("Mustermann", "Max", "", "0", "7", "0000-00-00", "", "0", "", "0", "", "", ""))
		w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
	t.Run("address id without row", func(t *testing.T) {
		newDB(t).
			on("select count(*) from person", rowOf("1")).
			on(personMainQuery, personRow("0", "0")).
			on("select uuid from adresse where id = 7", noRows(1))
		w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "no rows")
	})
}

// aqaa: suspected bug — the error of the main Scan is only checked at the very
// end. When the select fails, the handler aborts earlier on the empty sex value
// and returns the JSON of an errors.New value, i.e. `{}`, so the database error
// (B17: 500 for DB errors) is never reported to the client.
func TestIntegrationGetPersonReportsScanError(t *testing.T) {
	t.Skip("aqaa: open bug, see report")
	newDB(t).
		on("select count(*) from person", rowOf("1")).
		on(personMainQuery, dbFail("lost connection"))
	w := callAPI(http.MethodGet, "/api/persons/"+itPersonUUID, "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "lost connection")
}

// ---------------------------------------------------------------------------
// t9: PUT /api/persons/:pers_uuid (B20)

func samplePerson() DTOPerson {
	return DTOPerson{
		UUID:          uuidMust(itPersonUUID),
		FirstName:     "Seán",
		LastName:      "O'Connor",
		Title:         "Dr.",
		Sex:           "male",
		AddressUUID:   uuidMust(itAddressUUID),
		BirthDate:     civil(1980, 7, 5),
		BirthPlace:    "Bad Cannstatt",
		Nation:        "IRL",
		Privacy_State: true,
		Remarks:       "it's fine",
		FIDE_Nation:   "IRL",
	}
}

func TestIntegrationPutPersonInsert(t *testing.T) {
	db := newDB(t).
		on("select count(*) from person", rowOf("0")).
		on("SELECT id FROM adresse WHERE uuid = '"+itAddressUUID+"'", rowOf("7")).
		on("INSERT INTO person", affected(1))

	w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, mustJSON(t, samplePerson()))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	ins := db.one(t, "INSERT INTO person")
	// quotes escaped, title Dr. = 2, male = 1, address id 7, birthday without
	// zero padding (MySQL accepts the relaxed form), privacy 1, empty FIDE id
	// becomes a literal unquoted NULL
	assert.Contains(t, ins.query,
		"VALUES ('"+itPersonUUID+"', 'O\\'Connor', 'Seán', 2, 1, 7, '1980-7-5', 'Bad Cannstatt', 0, 'IRL', '1', 'it\\'s fine', 'IRL',NULL)")
	assert.Empty(t, db.all("UPDATE person"))
	assert.Empty(t, db.all("INSERT INTO adresse"))

	got := decodeBody[DTOPerson](t, w)
	assert.Equal(t, "Seán", got.FirstName)
	assert.Equal(t, "O'Connor", got.LastName)
}

func TestIntegrationPutPersonUpdate(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(p *DTOPerson)
		want   []string
	}{
		{
			name:   "empty fide id becomes NULL",
			mutate: func(p *DTOPerson) {},
			want: []string{
				"name = 'O\\'Connor',", "vorname = 'Seán',", "titel = '2',", "geschlecht = '1',", "adress = '7',",
				"geburtsdatum = '1980-7-5',", "datenschutz = '1',", "bemerkung = 'it\\'s fine',", "idfide = NULL",
				"WHERE uuid = '" + itPersonUUID + "'",
			},
		},
		{
			name:   "fide id is written unquoted",
			mutate: func(p *DTOPerson) { p.FIDE_Id = "4611016" },
			want:   []string{"idfide = 4611016\n"},
		},
		{
			name: "female, no title, no consent, dead, two digit date",
			mutate: func(p *DTOPerson) {
				p.Sex, p.Title, p.Privacy_State, p.Dead, p.BirthDate = "female", "", false, 1, civil(1999, 12, 31)
			},
			want: []string{"titel = '1',", "geschlecht = '0',", "datenschutz = '0',", "verstorben = '1',", "geburtsdatum = '1999-12-31',"},
		},
		{
			name:   "unknown sex and title",
			mutate: func(p *DTOPerson) { p.Sex, p.Title = "diverse", "Dipl.-Ing." },
			want:   []string{"titel = '5',", "geschlecht = '2',"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newDB(t).
				on("select count(*) from person", rowOf("1")).
				on("SELECT id FROM adresse WHERE uuid = '"+itAddressUUID+"'", rowOf("7")).
				on("UPDATE person set", affected(1))
			p := samplePerson()
			tc.mutate(&p)
			w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, mustJSON(t, p))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			upd := db.one(t, "UPDATE person set")
			for _, s := range tc.want {
				assert.Contains(t, upd.query, s)
			}
			assert.Empty(t, db.all("INSERT INTO person"))
		})
	}
}

func TestIntegrationPutPersonCreatesDummyAddress(t *testing.T) {
	db := newDB(t).
		on("select count(*) from person", rowOf("0")).
		on("SELECT id FROM adresse WHERE uuid = '"+itAddressUUID+"'", noRows(1), rowOf("9")).
		on("INSERT INTO adresse (uuid) VALUES ('"+itAddressUUID+"')", affected(1)).
		on("INSERT INTO person", affected(1))

	w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, mustJSON(t, samplePerson()))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	db.one(t, "INSERT INTO adresse (uuid)")
	assert.Contains(t, db.one(t, "INSERT INTO person").query, "'Seán', 2, 1, 9, '1980-7-5'")
}

func TestIntegrationPutPersonErrors(t *testing.T) {
	t.Run("invalid json", func(t *testing.T) {
		db := newDB(t)
		w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, `{"uuid":`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, db.queries())
	})
	t.Run("invalid uuid in body", func(t *testing.T) {
		db := newDB(t)
		w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, `{"uuid":"nope"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Empty(t, db.queries())
	})
	t.Run("url and body uuid differ", func(t *testing.T) {
		db := newDB(t)
		w := callAPI(http.MethodPut, "/api/persons/"+itOtherUUID, mustJSON(t, samplePerson()))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "does not fit")
		assert.Empty(t, db.queries())
	})
	t.Run("body without uuid", func(t *testing.T) {
		db := newDB(t)
		w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, `{"firstname":"Max"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), itZeroUUID)
		assert.Empty(t, db.queries())
	})
	t.Run("count query fails", func(t *testing.T) {
		db := newDB(t).on("select count(*) from person", dbFail("server has gone away"))
		w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, mustJSON(t, samplePerson()))
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "server has gone away")
		assert.Empty(t, db.all("INSERT"))
	})
	t.Run("duplicate uuid", func(t *testing.T) {
		db := newDB(t).
			on("select count(*) from person", rowOf("2")).
			on("SELECT id FROM adresse WHERE uuid", rowOf("7"))
		w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, mustJSON(t, samplePerson()))
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Empty(t, db.all("INSERT INTO person"))
		assert.Empty(t, db.all("UPDATE person"))
	})
	t.Run("dummy address cannot be created", func(t *testing.T) {
		db := newDB(t).
			on("select count(*) from person", rowOf("0")).
			on("SELECT id FROM adresse WHERE uuid", noRows(1)).
			on("INSERT INTO adresse (uuid)", dbFail("read only"))
		w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, mustJSON(t, samplePerson()))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "UUID was not found in table adresse")
		assert.Empty(t, db.all("INSERT INTO person"))
	})
	t.Run("insert fails", func(t *testing.T) {
		newDB(t).
			on("select count(*) from person", rowOf("0")).
			on("SELECT id FROM adresse WHERE uuid", rowOf("7")).
			on("INSERT INTO person", dbFail("Duplicate entry"))
		w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, mustJSON(t, samplePerson()))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Duplicate entry")
	})
	t.Run("update fails", func(t *testing.T) {
		newDB(t).
			on("select count(*) from person", rowOf("1")).
			on("SELECT id FROM adresse WHERE uuid", rowOf("7")).
			on("UPDATE person set", dbFail("Data too long"))
		w := callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, mustJSON(t, samplePerson()))
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Data too long")
	})
}

// aqaa: suspected bug — fide-id is concatenated into the INSERT/UPDATE without
// quotes and without validation, so a crafted fide-id becomes executable SQL.
func TestIntegrationPutPersonFideIDInjection(t *testing.T) {
	t.Skip("aqaa: open bug, see report")
	db := newDB(t).
		on("select count(*) from person", rowOf("0")).
		on("SELECT id FROM adresse WHERE uuid", rowOf("7")).
		on("INSERT INTO person", affected(1))
	p := samplePerson()
	p.FIDE_Id = "0); DELETE FROM person; -- "
	_ = callAPI(http.MethodPut, "/api/persons/"+itPersonUUID, mustJSON(t, p))
	for _, q := range db.queries() {
		assert.NotContains(t, q, "',0); DELETE FROM person;", "fide-id must not be embedded as raw SQL")
	}
}

// ---------------------------------------------------------------------------
// t10: deleteDTOGeneric via the DELETE routes (B19)

func TestIntegrationDeleteGeneric(t *testing.T) {
	routes := []struct {
		path string
		sql  string
	}{
		{path: "/api/persons/", sql: "delete from person where uuid = ?"},
		{path: "/api/federations/", sql: "delete from organisation where uuid = ?"},
		{path: "/api/clubs/", sql: "delete from organisation where uuid = ?"},
		{path: "/api/player-licences/", sql: "delete from mitgliedschaft where uuid = ?"},
		{path: "/api/club-officials/", sql: "delete from funktion where uuid = ?"},
	}
	cases := []struct {
		name       string
		result     fakeResult
		wantStatus int
		wantBody   string
	}{
		{name: "one row", result: affected(1), wantStatus: 200, wantBody: "1"},
		{name: "no row", result: affected(0), wantStatus: 404, wantBody: "0"},
		{name: "two rows", result: affected(2), wantStatus: 500, wantBody: "2"},
		{name: "db error", result: dbFail("foreign key constraint fails"), wantStatus: 500, wantBody: `"foreign key constraint fails"`},
	}
	for _, rt := range routes {
		for _, tc := range cases {
			t.Run(rt.path+tc.name, func(t *testing.T) {
				db := newDB(t).on("delete from", tc.result)
				w := callAPI(http.MethodDelete, rt.path+itPersonUUID, "")
				assert.Equal(t, tc.wantStatus, w.Code)
				assert.Equal(t, tc.wantBody, w.Body.String())
				del := db.one(t, "delete from")
				assert.Equal(t, rt.sql, del.query)
				assert.Equal(t, []driver.Value{itPersonUUID}, del.args)
			})
		}
		t.Run(rt.path+"invalid uuid", func(t *testing.T) {
			db := newDB(t)
			w := callAPI(http.MethodDelete, rt.path+"12345", "")
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "uuidParam is not valid: 12345")
			assert.Empty(t, db.queries())
		})
	}
}

func TestIntegrationDeleteAddressDispatch(t *testing.T) {
	t.Run("adressen", func(t *testing.T) {
		db := newDB(t).
			on("SELECT COUNT(*) from adressen", rowOf("1")).
			on("delete from adressen where uuid = ?", affected(1))
		w := callAPI(http.MethodDelete, "/api/addresses/"+itAddressUUID, "")
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, db.all("from adresse "))
	})
	t.Run("adresse", func(t *testing.T) {
		db := newDB(t).
			on("SELECT COUNT(*) from adressen", rowOf("0")).
			on("SELECT COUNT(*) from adresse ", rowOf("1")).
			on("delete from adresse where uuid = ?", affected(1))
		w := callAPI(http.MethodDelete, "/api/addresses/"+itAddressUUID, "")
		assert.Equal(t, http.StatusOK, w.Code)
		db.one(t, "delete from adresse where uuid = ?")
	})
	t.Run("unknown", func(t *testing.T) {
		db := newDB(t).on("SELECT COUNT(*) from", rowOf("0"))
		w := callAPI(http.MethodDelete, "/api/addresses/"+itAddressUUID, "")
		assert.Equal(t, http.StatusNotFound, w.Code)
		assert.Empty(t, db.all("delete"))
	})
}
