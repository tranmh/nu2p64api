package main

import (
	"database/sql"
	"flag"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

var (
	db                        *sql.DB
	yourMySQLdatabasepassword string
)

func init() {
	var err error
	db, err = sql.Open("mysql", "portal:Usm@1?/#Qv^avF@tcp(127.0.0.1:3306)/mvdsb")
	if err != nil {
		panic(err.Error())
	}
}

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
	UUID         uuid.UUID `json:"uuid"`
	Fedration_NR int       `json:"fedreation-nr"`
	Name         string    `json:"name"`
	NickName     string    `json:"nickname"`
	Region_UUID  uuid.UUID `json:"region-uuid"`
}

type DTOPerson struct {
	UUID      uuid.UUID `json:"uuid"`
	FirstName string    `json:"firstname"`
	LastName  string    `json:"lastname"`
	Title     string    `json:"title"`
	Gender    Gender    `json:"gender"`
	BirthYear int       `json:"birthyear"`
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

	if isValidUUID(uuidParam) {
		myUuid, _ := uuid.Parse(uuidParam)
		person.UUID = myUuid

		err := db.QueryRow("SELECT vorname, name FROM `person` where uuid = ?", myUuid).
			Scan(
				&person.FirstName,
				&person.LastName,
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
	uuidParam := c.Param("pers_uuid")

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

	if isValidUUID(person.UUID.String()) {
		myUuid, _ := uuid.Parse(person.UUID.String())

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

func getDTORegion(c *gin.Context) {
	// reg_uuid := c.Param("reg_uuid")
	var region DTORegion
	c.JSON(200, region)
}

func getDTOFederation(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	var federation DTOFederation
	c.JSON(200, federation)
}

func getDTOClub(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	var club DTOClub
	c.JSON(200, club)
}

func getDTOAddress(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	var address DTOAddress
	c.JSON(200, address)
}

func getDTOClubMember(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	// clubmem_uuid := c.Param("clubmem_uuid")
	var address DTOAddress
	c.JSON(200, address)
}

/*
func getDTOClubRole(c *gin.Context) {
	// role_uuid := c.Param("role_uuid")
	var clubrole DTOClubRole
	c.JSON(200, clubrole)
}
*/

func getDTOClubOfficial(c *gin.Context) {
	// fed_uuid := c.Param("fed_uuid")
	// club_uuid := c.Param("club_uuid")
	// clubmem_uuid := c.Param("clubmem_uuid")
	var clubofficial DTOClubOfficial
	c.JSON(200, clubofficial)
}

func main() {

	flag.StringVar(&yourMySQLdatabasepassword, "yourMySQLdatabasepassword", "NOT_SET", "your MySQL database password")

	flag.Parse()

	// var dataSourceName = "portal:" + yourMySQLdatabasepassword + "@tcp(127.0.0.1:3306)/mvdsb"  // FIXME, not in use

	router := gin.Default()

	authorized := router.Group("/", gin.BasicAuth(gin.Accounts{
		"anybody": "s3cr3t",
	}))

	authorized.GET("/regions/:reg_uuid", getDTORegion)

	authorized.GET("/federations/:fed_uuid", getDTOFederation)

	authorized.GET("/federations/:fed_uuid/clubs/:club_uuid", getDTOClub)

	authorized.GET("/federations/:fed_uuid/addresses/:addr_uuid", getDTOAddress)

	authorized.GET("/federations/:fed_uuid/persons/:pers_uuid", getDTOPerson)
	authorized.PUT("/federations/:fed_uuid/persons/:pers_uuid", putDTOPerson)
	authorized.DELETE("/federations/:fed_uuid/persons/:pers_uuid", deleteDTOPerson)

	authorized.GET("/federations/:fed_uuid/club/:club_uuid/member/:clubmem_uuid", getDTOClubMember)

	// authorized.GET("/clubRoles/:role_uuid", getDTOClubRole)

	authorized.GET("/federations/:fed_uuid/club/:club_uuid/officials/:role_uuid", getDTOClubOfficial)

	router.Run(":3030")
}
