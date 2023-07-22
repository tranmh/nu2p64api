package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

var (
	db                        *sql.DB
	yourMySQLdatabasepassword string
	basicAuthUsername         string
	basicAuthPassword         string
)

type Gender int

const (
	Female Gender = iota
	Male
	Unknown
)

func (s Gender) String() string {
	switch s {
	case Male:
		return "male"
	case Female:
		return "female"
	}
	return "unknown"
}

func getGender(gender string) Gender {
	if strings.Compare(gender, "female") == 0 {
		return Female
	} else if strings.Compare(gender, "male") == 0 {
		return Male
	} else {
		return Unknown
	}
}

type DTOAddress struct {
	UUID         uuid.UUID `json:"uuid"`
	Name         string    `json:"name"`
	Name2        string    `json:"name2"`
	Street       string    `json:"street"`
	ZIP          string    `json:"zip"`
	City         string    `json:"city"`
	Country      string    `json:"country"`
	Email        string    `json:"email"`        // mail.ParseAddress() to validate
	Email2       string    `json:"email2"`       // dito
	WWW          string    `json:"www"`          // url.ParseRequestURI to validate
	Phone_Mobile string    `json:"phone-mobile"` // github.com/dongri/phonenumber with phonenumber.Parse to validate
	Phone_Home   string    `json:"phone-home"`   // dito
	Phone_Work   string    `json:"phone-work"`   // dito
	Longitude    int       `json:"longitude"`
	Latitude     int       `json:"latitude"`
}

type DTOClub struct {
	UUID            uuid.UUID `json:"uuid"`
	Federation_UUID uuid.UUID `json:"federation-uuid"`
	Region_UUID     uuid.UUID `json:"region-uuid"`
	Club_NR         string    `json:"club-nr"`
	Name            string    `json:"name"`
	/*
		Entry_Date           time.Time   `json:"uuid"`
		Archived_Date        time.Time   `json:"uuid"`
		Contact_Address_UUID uuid.UUID   `json:"uuid"`
		Invoice_Address_UUID uuid.UUID   `json:"uuid"`
		Sport_Address_UUIDs  []uuid.UUID `json:"uuid"`
	*/
}

type DTOClubMember struct {
	UUID                uuid.UUID `json:"uuid"`
	Club_UUID           uuid.UUID `json:"club-uuid"`
	Person_UUID         uuid.UUID `json:"person-uuid"`
	Member_From         time.Time `json:"member-from"`
	Member_Until        time.Time `json:"member-until"`
	License_State       string    `json:"licence-state"` // ACTIVE, PASSIVE, NO_LICENSE
	License_Valid_From  time.Time `json:"license-valid-from"`
	License_Valid_Until time.Time `json:"license-valid-until"`
	Member_Nr           int       `json:"member-nr"`
}

type DTOClubOfficial struct {
	UUID        uuid.UUID `json:"uuid"`
	Club_UUID   uuid.UUID `json:"club-uuid"`
	Member_UUID uuid.UUID `json:"member-uuid"`
	Person_UUID uuid.UUID `json:"person-uuid"`
	Role_Name   string    `json:"role-name"`
	Valid_From  time.Time `json:"valid-from"`
	Valid_Until time.Time `json:"valid-until"`
}

type DTOFederation struct {
	UUID uuid.UUID `json:"uuid"`
	// Fedration_NR int       `json:"fedreation-nr"` // TODO, it is better to you string instead of int here, since we are using vkz C00 for Schachverband Württemberg
	Fedration_NR string    `json:"fedreation-nr"`
	Name         string    `json:"name"`
	NickName     string    `json:"nickname"`
	Region_UUID  uuid.UUID `json:"region-uuid"`
}

type DTOPerson struct {
	UUID      uuid.UUID `json:"uuid"`
	FirstName string    `json:"firstname"`
	LastName  string    `json:"lastname"`
	Title     string    `json:"title"`
	// Gender    Gender    `json:"gender"`
	Gender    string `json:"gender"`
	BirthYear int    `json:"birthyear"`
	// AddressUUID  string    `json:"gen"`
	// BirthDate    time.Time `json:"region-uuid"`
	// BirthPlace   string `json:"region-uuid"`
	Nation        string `json:"nation"`
	Privacy_State string `json:"privacy-state"`
	FIDE_Title    string `json:"fide-title"`
	FIDE_Nation   string `json:"fide-nation"`
	FIDE_Id       string `json:"fide-id"`
}

type DTORegion struct {
	UUID               uuid.UUID `json:"uuid"`
	Name               string    `json:"name"`
	NickName           string    `json:"nickname"`
	Pattern            string    `json:"pattern"`
	Parent_Region_UUID uuid.UUID `json:"parent-region-uuid"`
}

func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

// select
func getDTOPerson(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	uuidParam := c.Param("pers_uuid")

	var person DTOPerson
	var tmpBirthDay string

	if isValidUUID(uuidParam) {
		myUuid, _ := uuid.Parse(uuidParam)
		person.UUID = myUuid

		sqlSelectQuery := `
		select ifnull(person.name, ''), 
				ifnull(person.vorname, ''), 
				ifnull(titel.bezeichnung, ''), 
				ifnull(person.geschlecht, ''), 
				ifnull(person.geburtsdatum, ''), 
				ifnull(person.nation, ''), 
				ifnull(person.datenschutz, ''), 
				ifnull(person.nationfide, ''), 
				ifnull(person.idfide, '') 
		from person, titel 
		where person.titel=titel.id 
			and uuid = ?`

		err := db.QueryRow(sqlSelectQuery, myUuid).
			Scan(
				&person.FirstName,
				&person.LastName,
				&person.Title,
				&person.Gender,
				&tmpBirthDay,
				&person.Nation,
				&person.Privacy_State, // TODO: NULL, 0 or 1, 1 means accepted?
				// TODO we do not have FIDE_Title
				&person.FIDE_Nation,
				&person.FIDE_Id,
			)

		if strings.Compare(person.Gender, "0") == 0 {
			person.Gender = "female"
		} else if strings.Compare(person.Gender, "1") == 0 {
			person.Gender = "male"
		} else {
			c.JSON(500, errors.New("neither female=0 nor male=1 - broken data with gender aka person.geschlecht column? gender:"+person.Gender))
		}

		const layoutISO = "2006-01-02"
		t, parseBDError := time.Parse(layoutISO, tmpBirthDay)
		if parseBDError != nil {
			c.JSON(500, parseBDError.Error())
		} else {
			person.BirthYear = t.Year()
		}

		if err != nil {
			c.JSON(500, err.Error())
		} else {
			c.JSON(200, person)
		}
	} else {
		c.JSON(400, uuidParam)
	}
}

func deleteDTOGeneric(c *gin.Context, uuidParam string, deleteSQLStr string) {
	if isValidUUID(uuidParam) {
		myUuid, _ := uuid.Parse(uuidParam)

		result, err := db.Exec(deleteSQLStr, myUuid)

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

// delete
func deleteDTOPerson(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	uuidParam := c.Param("pers_uuid")
	deleteSQLStr := "delete from person where uuid = ?"
	deleteDTOGeneric(c, uuidParam, deleteSQLStr)
}

func convertTitleToTitleID(title string) int {
	if strings.Compare(title, "") == 0 {
		return 1
	} else if strings.Compare(title, "Dr") == 0 {
		return 2
	} else if strings.Compare(title, "Prof.") == 0 {
		return 3
	} else if strings.Compare(title, "Prof. Dr.") == 0 {
		return 4
	} else {
		return 5
	}
}

// upsert
func putDTOPerson(c *gin.Context) {
	var person DTOPerson
	err := c.BindJSON(&person)
	if err != nil {
		c.JSON(400, err)
		return
	}

	if isValidUUID(person.UUID.String()) {
		myUuid, parseErr := uuid.Parse(person.UUID.String())

		if parseErr != nil {
			c.JSON(400, parseErr)
			return
		}

		if !isValidUUID(myUuid.String()) {
			c.JSON(400, errors.New("myUuid is not a valid UUID: "+myUuid.String()))
			return
		}

		var count string
		var sqlSelectQuery string = `select count(*) from person where uuid = "` + myUuid.String() + `"`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		fmt.Printf(sqlSelectQuery)

		if errDBExec != nil {
			c.JSON(500, err.Error())
		} else {
			var title = convertTitleToTitleID(person.Title)
			var gender = getGender(person.Gender)
			var birthday = strconv.Itoa(person.BirthYear) + "-01-01"
			if strings.Compare(person.FIDE_Id, "") == 0 {
				person.FIDE_Id = "NULL"
			}

			if strings.Compare(count, "0") == 0 { // insert

				var sqlInsertQuery string = `
					INSERT INTO person (
						uuid,
						name, 
						vorname, 
						titel, 
						geschlecht, 
						geburtsdatum, 
						nation, 
						datenschutz, 
						nationfide, 
						idfide)
					VALUES ("` + person.UUID.String() +
					`", "` + person.LastName +
					`", "` + person.FirstName +
					`", "` + strconv.Itoa(title) +
					`", ` + strconv.Itoa(int(gender)) +
					`, "` + birthday +
					`", "` + person.Nation +
					`", "` + person.Privacy_State +
					`", "` + person.FIDE_Nation +
					`",` + person.FIDE_Id +
					`)
				`
				println(sqlInsertQuery)
				// TODO: missing columns for table person at insert: pkz, geburtsort, adress, gleichstellung, verstorben, etc. see:
				// select * from person where uuid = "aabe8313-f269-11ed-927b-005056054f4e";

				_, err3 := db.Exec(sqlInsertQuery)

				if err3 != nil {
					c.JSON(400, err3.Error())
				} else {
					c.JSON(200, person)
				}
			} else if strings.Compare(count, "1") == 0 { // update

				var sqlUpdateQuery string = `
					UPDATE person set 
						name = "` + person.LastName + `",
						vorname = "` + person.FirstName + `",
						titel = "` + strconv.Itoa(title) + `",
						geschlecht = "` + strconv.Itoa(int(gender)) + `",
						geburtsdatum = "` + birthday + `",
						nation = "` + person.Nation + `",
						datenschutz = "` + person.Privacy_State + `",
						nationfide = "` + person.FIDE_Nation + `",
						idfide = ` + person.FIDE_Id + `
					WHERE uuid = "` + person.UUID.String() + `"
				`
				println(sqlUpdateQuery)

				_, err4 := db.Exec(sqlUpdateQuery)
				if err4 != nil {
					c.JSON(400, err4.Error())
				} else {
					c.JSON(200, person)
				}
			} else {
				c.JSON(500, "panic, more than 1 federation with same uuid: "+myUuid.String())
			}
		}
	} else {
		c.JSON(400, errors.New("uuid is not valid"+person.UUID.String()))
	}
}

// table organisation for verein und verband
func getDTOFederation(c *gin.Context) {
	fed_uuid := c.Param("fed_uuid")
	var federation DTOFederation

	if isValidUUID(fed_uuid) {
		myUuid, _ := uuid.Parse(fed_uuid)
		federation.UUID = myUuid

		err := db.QueryRow("SELECT name, vkz FROM `organisation` where uuid = ?", myUuid).
			Scan(
				&federation.Name,
				&federation.Fedration_NR,
				// FIXME: not match of NickName and Region_UUID?
			)

		if err != nil {
			c.JSON(500, err.Error())
		} else {
			c.JSON(200, federation)
		}
	} else {
		c.JSON(400, fed_uuid)
	}
}

func putDTOFederation(c *gin.Context) {
	fed_uuid := c.Param("fed_uuid")

	var federation DTOFederation
	err := c.BindJSON(&federation)
	if err != nil {
		c.JSON(400, err)
		return
	}

	if strings.Compare(fed_uuid, federation.UUID.String()) != 0 {
		c.JSON(400, errors.New("uuid from URL and uuid as JSON in body does not fits: "+fed_uuid+" vs "+federation.UUID.String()))
		return
	}

	if isValidUUID(federation.UUID.String()) {
		myUuid, parseErr := uuid.Parse(federation.UUID.String())

		if parseErr != nil {
			c.JSON(400, parseErr)
			return
		}

		if !isValidUUID(myUuid.String()) {
			c.JSON(400, errors.New("myUuid is not a valid UUID: "+myUuid.String()))
			return
		}

		var count string
		var sqlSelectQuery string = `select count(*) from organisation where uuid = "` + myUuid.String() + `"`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		fmt.Printf(sqlSelectQuery)

		if errDBExec != nil {
			c.JSON(500, err.Error())
		} else {

			if strings.Compare(count, "0") == 0 { // insert

				var sqlInsertQuery string = `
					INSERT INTO organisation (
						uuid,
						name, 
						vkz)
					VALUES ("` + federation.UUID.String() +
					`", "` + federation.Name +
					`", "` + federation.Fedration_NR + `)
				`
				println(sqlInsertQuery)

				_, err3 := db.Exec(sqlInsertQuery)

				if err3 != nil {
					c.JSON(400, err3.Error())
				} else {
					c.JSON(200, federation)
				}
			} else if strings.Compare(count, "1") == 0 { // update

				var sqlUpdateQuery string = `
					UPDATE organisation set 
						name = "` + federation.Name + `",
						vkz = "` + federation.Fedration_NR + `"
					WHERE uuid = "` + federation.UUID.String() + `"
				`
				println(sqlUpdateQuery)

				_, err4 := db.Exec(sqlUpdateQuery)
				if err4 != nil {
					c.JSON(400, err4.Error())
				} else {
					c.JSON(200, federation)
				}
			} else {
				c.JSON(500, errors.New("panic, more than 1 federation with same uuid: "+myUuid.String()))
			}
		}
	} else {
		c.JSON(400, errors.New("uuid is not valid"+federation.UUID.String()))
	}
}

// table organisation as well
func getDTOClub(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	club_uuid := c.Param("club_uuid")
	var club DTOClub

	if isValidUUID(club_uuid) {
		myUuid, _ := uuid.Parse(club_uuid)
		club.UUID = myUuid

		err := db.QueryRow("SELECT name, vkz from `organisation` where uuid = ?", myUuid).
			Scan(
				&club.Name,
				&club.Club_NR,
				// TODO: region-uuid is not in use in portal64, but we use verband, unterverband, bezirk, verein.
			)

		if err != nil {
			c.JSON(500, err.Error())
			return
		}

		var vkzVerband string = string(club.Club_NR[0]) + "00"

		err2 := db.QueryRow("SELECT uuid from `organisation` where vkz = ?", vkzVerband).
			Scan(
				&club.Federation_UUID,
			)
		if err2 != nil {
			c.JSON(500, err2.Error())
			return
		}

		c.JSON(200, club)

	} else {
		c.JSON(400, club_uuid)
	}
}

func putDTOClub(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	club_uuid := c.Param("club_uuid")
	var club DTOClub
	err := c.BindJSON(&club)
	if err != nil {
		c.JSON(400, err)
		return
	}

	if strings.Compare(club_uuid, club.UUID.String()) != 0 {
		c.JSON(400, errors.New("uuid from URL and uuid as JSON in body does not fits: "+club_uuid+" vs "+club.UUID.String()))
		return
	}

	if isValidUUID(club.UUID.String()) {
		myUuid, parseErr := uuid.Parse(club.UUID.String())

		if parseErr != nil {
			c.JSON(400, parseErr)
			return
		}

		if !isValidUUID(myUuid.String()) {
			c.JSON(400, errors.New("myUuid is not a valid UUID: "+myUuid.String()))
			return
		}

		var count string
		var sqlSelectQuery string = `select count(*) from organisation where uuid = "` + myUuid.String() + `"`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		fmt.Printf(sqlSelectQuery)

		if errDBExec != nil {
			c.JSON(500, err.Error())
		} else {

			if strings.Compare(count, "0") == 0 { // insert

				var sqlInsertQuery string = `
					INSERT INTO organisation (
						uuid,
						name, 
						vkz)
					VALUES ("` + club.UUID.String() +
					`", "` + club.Name +
					`", "` + club.Club_NR + `)
				`
				println(sqlInsertQuery)

				_, err3 := db.Exec(sqlInsertQuery)

				if err3 != nil {
					c.JSON(400, err3.Error())
				} else {
					c.JSON(200, club)
				}
			} else if strings.Compare(count, "1") == 0 { // update

				var sqlUpdateQuery string = `
					UPDATE organisation set 
						name = "` + club.Name + `",
						vkz = "` + club.Club_NR + `"
					WHERE uuid = "` + club.UUID.String() + `"
				`
				println(sqlUpdateQuery)

				_, err4 := db.Exec(sqlUpdateQuery)
				if err4 != nil {
					c.JSON(400, err4.Error())
				} else {
					c.JSON(200, club)
				}
			} else {
				c.JSON(500, errors.New("panic, more than 1 club with same uuid: "+myUuid.String()))
			}
		}
	} else {
		c.JSON(400, errors.New("uuid is not valid"+club.UUID.String()))
	}
}

func deleteDTOClub(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	club_uuid := c.Param("club_uuid")
	deleteSQLStr := "delete from organisation where uuid = ?"
	deleteDTOGeneric(c, club_uuid, deleteSQLStr)
}

// table adresse, adressen and adr
func getDTOAddress(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	var address DTOAddress
	c.JSON(501, address)
}

func putDTOAddress(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	var address DTOAddress
	c.JSON(501, address)
}

func deleteDTOAddress(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	address_uuid := c.Param("addr_uuid")
	deleteSQLStr := "delete from adressen where uuid = ?" // TODO, ask Holger: what about table adr? What about table adresse?
	deleteDTOGeneric(c, address_uuid, deleteSQLStr)
}

// table person and table mitgliedschaft
func getDTOClubMember(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	// clubmem_uuid := c.Param("clubmem_uuid")
	var clubmember DTOClubMember
	c.JSON(501, clubmember)
}

func putDTOClubMember(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	// clubmem_uuid := c.Param("clubmem_uuid")
	var clubmember DTOClubMember
	c.JSON(501, clubmember)
}

func deleteDTOClubMember(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	clubmem_uuid := c.Param("clubmem_uuid")
	deleteSQLStr := "delete from mitgliedschaft where uuid = ?"
	deleteDTOGeneric(c, clubmem_uuid, deleteSQLStr)
}

// table funktion
func getDTOClubOfficial(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	// role_uuid := c.Param("role_uuid")
	var clubofficial DTOClubOfficial
	c.JSON(501, clubofficial)
}

func putDTOClubOfficial(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	// role_uuid := c.Param("role_uuid")
	var clubofficial DTOClubOfficial
	c.JSON(501, clubofficial)
}

func deleteDTOClubOfficial(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	role_uuid := c.Param("role_uuid")
	deleteSQLStr := "delete from funktion where uuid = ?"
	deleteDTOGeneric(c, role_uuid, deleteSQLStr)
}

func main() {

	flag.StringVar(&yourMySQLdatabasepassword, "yourMySQLdatabasepassword", "NOT_SET", "your MySQL database password")
	flag.StringVar(&basicAuthUsername, "basicAuthUsername", "NOT_SET", "your username for http basic authentication accessing the API")
	flag.StringVar(&basicAuthPassword, "basicAuthPassword", "NOT_SET", "your password for http basic authentication accessing the API")

	flag.Parse()

	var dataSourceName = "portal:" + yourMySQLdatabasepassword + "@tcp(127.0.0.1:3306)/mvdsb"
	var err error
	db, err = sql.Open("mysql", dataSourceName)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	router := gin.Default()

	authorized := router.Group("/", gin.BasicAuth(gin.Accounts{
		basicAuthUsername: basicAuthPassword,
	}))

	// Not implemented, since there is no region in portal64 according to Holger, no implementation means 404 page not found
	// authorized.GET("/regions/:reg_uuid", getDTORegion)
	// authorized.PUT("/regions/:reg_uuid", putDTORegion)

	authorized.GET("/federations/:fed_uuid", getDTOFederation)
	authorized.PUT("/federations/:fed_uuid", putDTOFederation)

	authorized.GET("/federations/:fed_uuid/clubs/:club_uuid", getDTOClub)
	authorized.PUT("/federations/:fed_uuid/clubs/:club_uuid", putDTOClub)
	authorized.DELETE("/federations/:fed_uuid/clubs/:club_uuid", deleteDTOClub)

	authorized.GET("/federations/:fed_uuid/addresses/:addr_uuid", getDTOAddress)
	authorized.PUT("/federations/:fed_uuid/addresses/:addr_uuid", putDTOAddress)
	authorized.DELETE("/federations/:fed_uuid/addresses/:addr_uuid", deleteDTOAddress)

	authorized.GET("/federations/:fed_uuid/persons/:pers_uuid", getDTOPerson)
	authorized.PUT("/federations/:fed_uuid/persons/:pers_uuid", putDTOPerson)
	authorized.DELETE("/federations/:fed_uuid/persons/:pers_uuid", deleteDTOPerson)

	authorized.GET("/federations/:fed_uuid/club/:club_uuid/member/:clubmem_uuid", getDTOClubMember)
	authorized.PUT("/federations/:fed_uuid/club/:club_uuid/member/:clubmem_uuid", putDTOClubMember)
	authorized.DELETE("/federations/:fed_uuid/club/:club_uuid/member/:clubmem_uuid", deleteDTOClubMember)

	// Not implemented, hard coded, not in the data base and changable, no implementation means 404 page not found
	// authorized.GET("/clubRoles/:role_uuid", getDTOClubRole)

	authorized.GET("/federations/:fed_uuid/club/:club_uuid/officials/:role_uuid", getDTOClubOfficial)
	authorized.PUT("/federations/:fed_uuid/club/:club_uuid/officials/:role_uuid", putDTOClubOfficial)
	authorized.DELETE("/federations/:fed_uuid/club/:club_uuid/officials/:role_uuid", deleteDTOClubOfficial)

	router.Run(":3030")
}
