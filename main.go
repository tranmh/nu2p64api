package main

import (
	"database/sql"
	"flag"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

var db sql.DB

var (
	yourMySQLdatabasepassword string
)

type Gender int

const (
	Male Gender = iota
	Female
)

func (s Gender) String() string {
	switch s {
	case Male:
		return "Male"
	case Female:
		return "Female"
	}
	return "unknown"
}

type DTOAddress struct {
	uuid         uuid.UUID
	name         string
	name2        string
	street       string
	zip          string
	city         string
	email        string // mail.ParseAddress() to validate
	email2       string // dito
	www          string // url.ParseRequestURI to validate
	phone_mobile string // github.com/dongri/phonenumber with phonenumber.Parse to validate
	phone_home   string // dito
	phone_work   string // dito
}

type DTOClub struct {
	uuid                 uuid.UUID
	federation_uuid      uuid.UUID
	region_uuid          uuid.UUID
	club_nr              string
	name                 string
	entry_date           time.Time
	archived_date        time.Time
	contact_address_uuid uuid.UUID
	invoice_address_uuid uuid.UUID
	sport_address_uuids  []uuid.UUID
}

type DTOClubMember struct {
	uuid                uuid.UUID
	club_uuid           uuid.UUID
	person_uuid         uuid.UUID
	member_from         time.Time
	member_until        time.Time
	license_state       string
	license_valid_from  time.Time
	license_valid_until time.Time
}

type DTOClubOfficial struct {
	uuid           uuid.UUID
	club_uuid      uuid.UUID
	member_uuid    uuid.UUID
	person_uuid    uuid.UUID
	club_role_uuid uuid.UUID
	valid_from     time.Time
	valid_until    time.Time
}

type DTOClubRole struct {
	uuid     uuid.UUID
	name     string
	nickname string
}

type DTOFederation struct {
	uuid          uuid.UUID
	federation_nr int
	name          string
	nickname      string
	region_uuid   uuid.UUID
}

type DTOPerson struct {
	uuid          uuid.UUID
	firstname     string
	lastname      string
	gender        Gender
	address_uuid  string
	birthdate     time.Time
	birthyear     int
	birthplace    string
	privacy_state string
}

type DTORegion struct {
	uuid               uuid.UUID
	name               string
	nickname           string
	pattern            string
	parent_region_uuid uuid.UUID
}

func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

// select
func getDTOPerson(c *gin.Context) {
	uuidParam := c.Param("uuid")

	var person DTOPerson

	if isValidUUID(uuidParam) {
		myUuid, _ := uuid.Parse(uuidParam)
		person.uuid = myUuid

		err := db.QueryRow("SELECT vorname, name, geschlecht, geburtsdatum, geburtsort FROM `person` where uuid = ?", myUuid).
			Scan(
				&person.firstname,
				&person.lastname,
				&person.gender,
				&person.birthdate,
				&person.birthplace,
			)

		if err != nil {
			c.JSON(500, err.Error())
		} else {
			c.JSON(200, person)
		}
	} else {
		c.JSON(400, uuidParam)
	}
}

// delete
func deleteDTOPerson(c *gin.Context) {
	uuidParam := c.Param("uuid")

	if isValidUUID(uuidParam) {
		myUuid, _ := uuid.Parse(uuidParam)

		result, err := db.Exec("delete from person where uuid = ?", myUuid)

		if err != nil {
			c.JSON(500, err.Error())
		} else {
			rowsAffected, err2 := result.RowsAffected()
			if err2 != nil {
				c.JSON(500, err2.Error())
			} else if rowsAffected != 1 {
				c.JSON(500, rowsAffected)
			} else {
				c.JSON(200, rowsAffected)
			}
		}
	} else {
		c.JSON(400, uuidParam)
	}
}

// upsert
func putDTOPerson(c *gin.Context) {
	var person DTOPerson
	c.BindJSON(&person)

	if isValidUUID(person.uuid.String()) {
		myUuid, _ := uuid.Parse(person.uuid.String())

		result, err := db.Exec("select * from person where uuid = ?", myUuid)

		if err != nil {
			c.JSON(500, err.Error())
		} else {
			rowsAffected, err2 := result.RowsAffected()
			if err2 != nil {
				c.JSON(500, err2.Error())
			} else if rowsAffected == 0 {
				// TODO: insert
			} else if rowsAffected == 1 {
				// TODO: update
			} else {
				c.JSON(500, "panic")
			}
		}
	} else {
		c.JSON(400, person)
	}
}

func main() {

	flag.StringVar(&yourMySQLdatabasepassword, "yourMySQLdatabasepassword", "NOT_SET", "your MySQL database password")

	flag.Parse()

	var dataSourceName = "portal:" + yourMySQLdatabasepassword + "@tcp(127.0.0.1:3306)/mvdsb"
	db, err := sql.Open("mysql", dataSourceName)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	router := gin.Default()

	authorized := router.Group("/", gin.BasicAuth(gin.Accounts{
		"anybody": "s3cr3t",
	}))

	authorized.GET("/person/:uuid", getDTOPerson)
	authorized.PUT("/person/", putDTOPerson)
	authorized.DELETE("/person/:uuid", deleteDTOPerson)

	router.Run(":3030")
}
