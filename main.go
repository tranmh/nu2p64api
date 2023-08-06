package main

import (
	"bytes"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	log "github.com/sirupsen/logrus"
)

var (
	db                        *sql.DB
	yourMySQLdatabasepassword string
	basicAuthUsername         string
	basicAuthPassword         string
)

// -----------------------------------------------------------------------------
// https://medium.com/pengenpaham/implement-basic-logging-with-gin-and-logrus-5f36fba69b28

func LoggingMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Starting time
		startTime := time.Now()

		// Processing request
		// ctx.Next() // bug?

		// End Time
		endTime := time.Now()

		// execution time
		latencyTime := endTime.Sub(startTime)

		// Request method
		reqMethod := ctx.Request.Method

		// Request route
		reqUri := ctx.Request.RequestURI

		// status code
		statusCode := ctx.Writer.Status()

		// Request IP
		clientIP := ctx.ClientIP()

		reqBody, _ := ioutil.ReadAll(ctx.Request.Body)
		ctx.Request.Body = ioutil.NopCloser(bytes.NewReader(reqBody))

		log.WithFields(log.Fields{
			"6_BODY":      string(reqBody),
			"5_METHOD":    reqMethod,
			"2_URI":       reqUri,
			"3_STATUS":    statusCode,
			"4_LATENCY":   latencyTime,
			"1_CLIENT_IP": clientIP,
		}).Info("HTTP REQUEST")

		ctx.Next()
	}
}

func initLog() {
	// load environment variable
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// setup logrus
	logLevel, err := log.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logLevel = log.InfoLevel
	}

	log.SetLevel(logLevel)
	log.SetFormatter(&log.JSONFormatter{})
}

// -----------------------------------------------------------------------------
// https://seefnasrul.medium.com/create-your-first-go-rest-api-with-jwt-authentication-in-gin-framework-dbe5bda72817

type LoginUser struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// for now we do not do anything yet with registering user
func registerLoginUser(c *gin.Context) {

	var input LoginUser

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "validated!"})

}

func verifyPassword(username string, password string) bool {
	envFile, _ := godotenv.Read(".env")

	if (strings.Compare(username, envFile["JWT_USERNAME"]) == 0) &&
		(strings.Compare(password, envFile["JWT_PASSWORD"]) == 0) {
		return true
	} else {
		return false
	}
}

func loginCheck(username string, password string) (string, error) {

	var err error

	if !verifyPassword(username, password) {
		return "", errors.New("username or password is incorrect.")
	}

	token, err := GenerateToken(1) // TODO, only one user with one user id for now

	if err != nil {
		return "", err
	}

	return token, nil

}

func GenerateToken(user_id uint) (string, error) {

	envFile, _ := godotenv.Read(".env")

	token_lifespan, err := strconv.Atoi(envFile["TOKEN_HOUR_LIFESPAN"])

	if err != nil {
		return "", err
	}

	claims := jwt.MapClaims{}
	claims["authorized"] = true
	claims["user_id"] = user_id
	claims["exp"] = time.Now().Add(time.Hour * time.Duration(token_lifespan)).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(envFile["API_SECRET"]))
}

func tokenValid(c *gin.Context) error {

	envFile, _ := godotenv.Read(".env")

	tokenString := extractToken(c)
	_, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(envFile["API_SECRET"]), nil
	})
	if err != nil {
		return err
	}
	return nil
}

func extractToken(c *gin.Context) string {
	token := c.Query("token")
	if token != "" {
		return token
	}
	bearerToken := c.Request.Header.Get("Authorization")
	if len(strings.Split(bearerToken, " ")) == 2 {
		return strings.Split(bearerToken, " ")[1]
	}
	return ""
}

func loginUser(c *gin.Context) {

	var input LoginUser

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := loginCheck(input.Username, input.Password)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username or password is incorrect."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})

}

func jwtAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := tokenValid(c)
		if err != nil {
			c.String(http.StatusUnauthorized, "Unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

// https://romangaranin.net/posts/2021-02-19-json-time-and-golang/
// -----------------------------------------------------------------------------
type CivilTime time.Time

func (c *CivilTime) UnmarshalJSON(b []byte) error {
	value := strings.Trim(string(b), `"`) //get rid of "
	if value == "" || value == "null" {
		return nil
	}

	t, err := time.Parse("2006-01-02", value) //parse time
	if err != nil {
		return err
	}
	*c = CivilTime(t) //set result using the pointer
	return nil
}

func (c CivilTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(c).Format("2006-01-02") + `"`), nil
}

// -----------------------------------------------------------------------------

type Gender int

const (
	Female Gender = iota
	Male
	GenderUnknown
)

func (s Gender) String() string {
	switch s {
	case Male:
		return "male"
	case Female:
		return "female"
	}
	return "gender unknown"
}

func getGender(gender string) Gender {
	if strings.Compare(gender, "female") == 0 {
		return Female
	} else if strings.Compare(gender, "male") == 0 {
		return Male
	} else {
		return GenderUnknown
	}
}

type PlayerLicenseRequestType int

const (
	Active PlayerLicenseRequestType = iota
	Passive
	Club_Transfer
	Switch
	Delete
	PlayerLicenseRequestTypeUnknown
)

func getPlayerLicenseRequestType(requestType string) PlayerLicenseRequestType {
	if strings.Compare(requestType, "ACTIVE") == 0 {
		return Active
	} else if strings.Compare(requestType, "PASSIVE") == 0 {
		return Passive
	} else if strings.Compare(requestType, "CLUB_TRANSFER") == 0 {
		return Club_Transfer
	} else if strings.Compare(requestType, "SWITCH") == 0 {
		return Switch
	} else if strings.Compare(requestType, "DELETE") == 0 {
		return Delete
	} else {
		return PlayerLicenseRequestTypeUnknown
	}
}

// -----------------------------------------------------------------------------

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
	Longitude    float64   `json:"longitude"`
	Latitude     float64   `json:"latitude"`
}

type DTOClub struct {
	UUID                        uuid.UUID   `json:"uuid"`
	Federation_UUID             uuid.UUID   `json:"federation-uuid"`
	Region_UUID                 uuid.UUID   `json:"region-uuid"`
	Club_NR                     string      `json:"club-nr"`
	Name                        string      `json:"name"`
	Entry_Date                  CivilTime   `json:"entry-date"`
	Contact_Address_UUID        uuid.UUID   `json:"contact-address-uuid"`
	Sport_Address_UUIDs         []uuid.UUID `json:"sport-address-uuids"`
	Register_Of_Associations_Nr string      `json:"register-of-associations-nr"`
	Club_Type                   string      `json:"club-type"`
	Bank_Account_Owner          string      `json:"bank-account-owner"`
	Bank_Account_Bank           string      `json:"bank-account-bank"`
	Bank_Account_BIC            string      `json:"bank-account-big"`
	Bank_Account_IBAN           string      `json:"bank-account-iban"`
	Direct_Debit                bool        `json:"direct-debit"`
}

type DTOClubMember struct {
	UUID                uuid.UUID `json:"uuid"`
	Club_UUID           uuid.UUID `json:"club-uuid"`
	Person_UUID         uuid.UUID `json:"person-uuid"`
	Member_From         CivilTime `json:"member-from"`
	Member_Until        CivilTime `json:"member-until"`
	License_State       string    `json:"licence-state"` // ACTIVE, PASSIVE, NO_LICENSE
	License_Valid_From  CivilTime `json:"license-valid-from"`
	License_Valid_Until CivilTime `json:"license-valid-until"`
	Member_Nr           int       `json:"member-nr"`
}

type DTOClubOfficial struct {
	UUID        uuid.UUID `json:"uuid"`
	Club_UUID   uuid.UUID `json:"club-uuid"`
	Member_UUID uuid.UUID `json:"member-uuid"`
	Person_UUID uuid.UUID `json:"person-uuid"`
	Role_Name   string    `json:"role-name"`
	Valid_From  CivilTime `json:"valid-from"`
	Valid_Until CivilTime `json:"valid-until"`
}

type DTOFederation struct {
	UUID         uuid.UUID `json:"uuid"`
	Fedration_NR string    `json:"fedreation-nr"`
	Name         string    `json:"name"`
	NickName     string    `json:"nickname"`
	Region_UUID  uuid.UUID `json:"region-uuid"`
}

type DTOPerson struct {
	UUID          uuid.UUID `json:"uuid"`
	FirstName     string    `json:"firstname"`
	LastName      string    `json:"lastname"`
	Title         string    `json:"title"`
	Gender        string    `json:"gender"`
	AddressUUID   uuid.UUID `json:"address-uuid"`
	BirthDate     CivilTime `json:"birthdate"`
	BirthPlace    string    `json:"birthplace"`
	BirthName     string    `json:"birthname"`
	Dead          int       `json:"dead"`
	Nation        string    `json:"nation"`
	Privacy_State string    `json:"privacy-state"`
	Remarks       string    `json:"remarks"`
	FIDE_Title    string    `json:"fide-title"`
	FIDE_Nation   string    `json:"fide-nation"`
	FIDE_Id       string    `json:"fide-id"`
}

type DTORegion struct {
	UUID               uuid.UUID `json:"uuid"`
	Name               string    `json:"name"`
	NickName           string    `json:"nickname"`
	Pattern            string    `json:"pattern"`
	Parent_Region_UUID uuid.UUID `json:"parent-region-uuid"`
}

type DTOPlayerLicense struct {
	UUID              uuid.UUID                `json:"uuid"`
	Club_UUID         uuid.UUID                `json:"club-uuid"`
	Prev_Club_UUID    uuid.UUID                `json:"prev-club-uuid"`
	Person_UUID       uuid.UUID                `json:"person-uuid"`
	RequestDate       CivilTime                `json:"request-date"`
	RequestType       PlayerLicenseRequestType `json:"request_type"`
	LicenseValidFrom  CivilTime                `json:"license-valid-from"`
	LicenseValidUntil CivilTime                `json:"license-valid-until"`
	LicenseState      string                   `json:"license-state"` // ACTIVE, PASSIVE
	MemberNr          int                      `json:"pkz"`           // PKZ
}

// -----------------------------------------------------------------------------

func init() {
	initLog()
}

func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

// select
func getDTOPerson(c *gin.Context) {
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
				// we do not have FIDE_Title, so ignore
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
			person.BirthDate = CivilTime(t)
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
	uuidParam := c.Param("pers_uuid")
	deleteSQLStr := "delete from person where uuid = ?"
	deleteDTOGeneric(c, uuidParam, deleteSQLStr)
}

func convertTitleToTitleID(title string) int {
	if strings.Compare(title, "") == 0 {
		return 1
	} else if strings.Compare(title, "Dr.") == 0 {
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
			var birthday = strconv.Itoa(time.Time(person.BirthDate).Year()) + "-" + strconv.Itoa(int(time.Time(person.BirthDate).Month())) + "-" + strconv.Itoa(time.Time(person.BirthDate).Day())
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
					`", "` + federation.Fedration_NR + `")
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
					`", "` + club.Club_NR + `")
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
	club_uuid := c.Param("club_uuid")
	deleteSQLStr := "delete from organisation where uuid = ?"
	deleteDTOGeneric(c, club_uuid, deleteSQLStr)
}

// TODO, what happens to address of persons? This route at the moment is for club only. Get it from Table adresse
func getDTOAddress(c *gin.Context) {
	addr_uuid := c.Param("addr_uuid")
	var address DTOAddress

	if isValidUUID(addr_uuid) {

		address.UUID, _ = uuid.Parse(addr_uuid)

		var sqlQuerySelect = `
			SELECT 
				ifnull(adr.wert, ''),
				ifnull(adr_art.id, '')
			FROM adressen, adr, adr_art
			WHERE
				adressen.id = adr.id_adressen AND
				adr_art.id = adr.id_art AND
				adressen.uuid = "` + addr_uuid + "\""

		fmt.Println(sqlQuerySelect)

		rows, err := db.Query(sqlQuerySelect)
		if err != nil {
			c.JSON(400, err)
			return
		}
		defer rows.Close()

		var addrValue string
		var addrId int
		for rows.Next() {
			err := rows.Scan(&addrValue, &addrId)
			if err != nil {
				c.JSON(401, err.Error())
				return
			}

			if addrId == 2 {
				address.Street = addrValue
			}
			if addrId == 3 {
				address.ZIP = addrValue
			}
			if addrId == 4 {
				address.City = addrValue
			}
			if addrId == 5 {
				address.Country = addrValue
			}
			if addrId == 6 {
				address.Phone_Home = addrValue
			}
			if addrId == 7 {
				address.Phone_Mobile = addrValue
			}
			if addrId == 8 {
				address.Phone_Work = addrValue
			}
			// 9 skip fax
			if addrId == 10 {
				address.Email = addrValue
			}
			if addrId == 11 {
				address.Email2 = addrValue
			}
			if addrId == 15 {
				address.WWW = addrValue
			}
			if addrId == 17 {
				address.Latitude, err = strconv.ParseFloat(addrValue, 64)
				if err != nil {
					c.JSON(500, err)
				}
			}
			if addrId == 18 {
				address.Longitude, err = strconv.ParseFloat(addrValue, 64)
				if err != nil {
					c.JSON(500, err)
				}
			}
		}
		if err := rows.Err(); err != nil {
			c.JSON(402, err)
			return
		}

		c.JSON(200, address)

	} else {
		c.JSON(403, addr_uuid)
	}
}

// TODO convert all error c.JSON(400, parseErr) to c.JSON(400, parseErr.Error())

func updateAdrTableWithValue(addrValue string, id_address int, id_art int, c *gin.Context) {
	var sqlUpdateQuery = `
	UPDATE adr set 
		wert = "` + addrValue + `"
	WHERE id_adressen = ` + strconv.Itoa(id_address) + ` AND
		id_art = ` + strconv.Itoa(id_art)
	println(sqlUpdateQuery)
	_, err := db.Exec(sqlUpdateQuery)
	if err != nil {
		c.JSON(400, err.Error())
	}
}

func putDTOAddress(c *gin.Context) {
	addr_uuid := c.Param("addr_uuid")
	var addressOfClub DTOAddress

	err := c.BindJSON(&addressOfClub)
	if err != nil {
		c.JSON(400, err)
		return
	}

	if strings.Compare(addr_uuid, addressOfClub.UUID.String()) != 0 {
		c.JSON(400, errors.New("uuid from URL and uuid as JSON in body does not fits: "+addr_uuid+" vs "+addressOfClub.UUID.String()))
		return
	}

	if isValidUUID(addressOfClub.UUID.String()) {
		myUuid, _ := uuid.Parse(addressOfClub.UUID.String())

		var count string
		var sqlSelectQuery string = `select count(*) from adressen where uuid = "` + myUuid.String() + `"`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		fmt.Printf(sqlSelectQuery)

		if errDBExec != nil {
			c.JSON(500, err.Error())
		} else {

			if strings.Compare(count, "0") == 0 { // insert

				// TODO: to whome does this address belongs to in case of insert?

				var sqlInsertQuery string = `
					FIXME  to whome does this address belongs to in case of insert INSERT INTO adressen (
						uuid)
					VALUES ("` + addressOfClub.UUID.String() + `")
				`
				println(sqlInsertQuery)

				_, err3 := db.Exec(sqlInsertQuery)

				if err3 != nil {
					c.JSON(400, err3.Error())
				} else {
					c.JSON(200, addressOfClub)
				}
			} else if strings.Compare(count, "1") == 0 { // update

				var tmpIdAddress, _ = getIDFromUUID("adressen", addressOfClub.UUID)

				updateAdrTableWithValue(addressOfClub.Street, tmpIdAddress, 2, c)
				updateAdrTableWithValue(addressOfClub.ZIP, tmpIdAddress, 3, c)
				updateAdrTableWithValue(addressOfClub.City, tmpIdAddress, 4, c)
				updateAdrTableWithValue(addressOfClub.Country, tmpIdAddress, 5, c)
				updateAdrTableWithValue(addressOfClub.Phone_Home, tmpIdAddress, 6, c)
				updateAdrTableWithValue(addressOfClub.Phone_Mobile, tmpIdAddress, 7, c)
				updateAdrTableWithValue(addressOfClub.Phone_Work, tmpIdAddress, 8, c)
				updateAdrTableWithValue(addressOfClub.Email, tmpIdAddress, 10, c)
				updateAdrTableWithValue(addressOfClub.Email2, tmpIdAddress, 11, c)
				updateAdrTableWithValue(addressOfClub.WWW, tmpIdAddress, 15, c)
				updateAdrTableWithValue(fmt.Sprintf("%v", addressOfClub.Latitude), tmpIdAddress, 17, c)
				updateAdrTableWithValue(fmt.Sprintf("%v", addressOfClub.Longitude), tmpIdAddress, 18, c)

				c.JSON(200, addressOfClub)
			} else {
				c.JSON(500, errors.New("panic, more than 1 address with same uuid: "+myUuid.String()))
			}
		}
	} else {
		c.JSON(400, errors.New("uuid is not valid"+addressOfClub.UUID.String()))
	}
}

func deleteDTOAddress(c *gin.Context) {
	address_uuid := c.Param("addr_uuid")
	deleteSQLStr := "delete from adressen where uuid = ?"
	deleteDTOGeneric(c, address_uuid, deleteSQLStr)
}

// table person and table mitgliedschaft
func getDTOClubMember(c *gin.Context) {
	club_uuid := c.Param("club_uuid")
	clubmem_uuid := c.Param("clubmem_uuid")
	var clubmember DTOClubMember

	if isValidUUID(clubmem_uuid) {
		myUuid, _ := uuid.Parse(clubmem_uuid)
		clubmember.UUID = myUuid

		// TODO: ask Holger. Semantic behind status, stat1 and stat2.
		// TODO: ask Nu, how to map member-from, member-until, license-valid-from, license-valid-until
		var sqlSelectQuery string = `
			SELECT ifnull(organisation.uuid, ''), 
				ifnull(person.uuid, ''), 
				ifnull(mitgliedschaft.von, ''), 
				ifnull(mitgliedschaft.bis, ''), 
				ifnull(mitgliedschaft.stat1, ''),
				ifnull(mitgliedschaft.spielernummer, '')
			FROM mitgliedschaft, 
				organisation,
				person
			WHERE mitgliedschaft.organisation = organisation.id AND 
				mitgliedschaft.person = person.id AND
				mitgliedschaft.uuid = "` + clubmem_uuid + `"`
		fmt.Println(sqlSelectQuery)

		var memberFrom string
		var memberUntil string

		err := db.QueryRow(sqlSelectQuery).
			Scan(
				&clubmember.Club_UUID,
				&clubmember.Person_UUID,
				&memberFrom,
				&memberUntil,
				&clubmember.License_State,
				&clubmember.Member_Nr,
			)

		fmt.Println(memberFrom)
		fmt.Println(memberUntil)
		const layoutISO = "2006-01-02"
		if strings.Compare(memberFrom, "") != 0 {
			tMemberFrom, parseBDError := time.Parse(layoutISO, memberFrom)
			if parseBDError != nil {
				c.JSON(500, parseBDError.Error())
				return
			} else {
				clubmember.Member_From = CivilTime(tMemberFrom)
			}
		}
		if strings.Compare(memberUntil, "") != 0 {
			tMemberUntil, parseBDError2 := time.Parse(layoutISO, memberUntil)
			if parseBDError2 != nil {
				c.JSON(500, parseBDError2.Error())
				return
			} else {
				clubmember.Member_Until = CivilTime(tMemberUntil)
			}
		}

		if strings.Compare(club_uuid, clubmember.Club_UUID.String()) != 0 {
			c.JSON(400, "club_uuid as URL does not fit to the content in the database: "+club_uuid+" vs "+clubmember.Club_UUID.String())
			return
		}

		if err != nil {
			c.JSON(500, err.Error())
			return
		}

		c.JSON(200, clubmember)

	} else {
		c.JSON(400, errors.New("uuid is not valid "+clubmem_uuid))
	}
}

func getUUIDFromID(tableName string, id int) (rUuid uuid.UUID, rErr error) {
	var sqlQuerySelectUUID = "select uuid from " + tableName + " where id = " + strconv.Itoa(id)
	var tmpUUID string
	rErr = db.QueryRow(sqlQuerySelectUUID).Scan(&tmpUUID)
	if rErr == nil {
		rUuid, parseErr := uuid.Parse(tmpUUID)
		if parseErr == nil {
			return rUuid, nil
		} else {
			return rUuid, parseErr
		}
	} else {
		return rUuid, rErr
	}
}

func getIDFromUUID(tableName string, myUuid uuid.UUID) (id int, rErr error) {
	var sqlQuerySelectID = "select id from " + tableName + " where uuid = \"" + myUuid.String() + "\""
	var tmpId int
	rErr = db.QueryRow(sqlQuerySelectID).Scan(&tmpId)
	return tmpId, rErr
}

func putDTOClubMember(c *gin.Context) {
	clubmem_uuid := c.Param("clubmem_uuid")
	var clubmember DTOClubMember

	err := c.BindJSON(&clubmember)
	if err != nil {
		c.JSON(400, err)
		return
	}

	if strings.Compare(clubmem_uuid, clubmember.UUID.String()) != 0 {
		c.JSON(400, errors.New("uuid from URL and uuid as JSON in body does not fits: "+clubmem_uuid+" vs "+clubmember.UUID.String()))
		return
	}

	if isValidUUID(clubmember.UUID.String()) {
		myUuid, parseErr := uuid.Parse(clubmember.UUID.String())

		if parseErr != nil {
			c.JSON(400, parseErr)
			return
		}

		if !isValidUUID(myUuid.String()) {
			c.JSON(400, errors.New("myUuid is not a valid UUID: "+myUuid.String()))
			return
		}

		var count string
		var sqlSelectQuery string = `select count(*) from mitgliedschaft where uuid = "` + myUuid.String() + `"`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		fmt.Printf(sqlSelectQuery)

		if errDBExec != nil {
			c.JSON(500, err.Error())
		} else {

			var person_id, _ = getIDFromUUID("person", clubmember.Person_UUID)
			var organisation_id, _ = getIDFromUUID("organisation", clubmember.Club_UUID)
			var fromStr = time.Time(clubmember.Member_From).Format("2006-01-02")
			var untilStr = time.Time(clubmember.Member_Until).Format("2006-01-02")

			if strings.Compare(count, "0") == 0 { // insert

				var sqlInsertQuery string = `
					INSERT INTO mitgliedschaft (
						uuid,
						person,
						organisation,
						von,
						bis,
						spielernummer)
					VALUES ("` + clubmember.UUID.String() +
					`", ` + strconv.Itoa(person_id) +
					`, "` + strconv.Itoa(organisation_id) +
					`", "` + fromStr +
					`", "` + untilStr +
					`", ` + strconv.Itoa(clubmember.Member_Nr) + `)
				`
				println(sqlInsertQuery)

				_, err3 := db.Exec(sqlInsertQuery)

				if err3 != nil {
					c.JSON(400, err3.Error())
				} else {
					c.JSON(200, clubmember)
				}
			} else if strings.Compare(count, "1") == 0 { // update

				var sqlUpdateQuery string = `
					UPDATE mitgliedschaft set 
						person = ` + strconv.Itoa(person_id) + `,
						organisation = ` + strconv.Itoa(organisation_id) + `,
						von = "` + fromStr + `",
						bis = "` + untilStr + `",
						spielernummer = ` + strconv.Itoa(clubmember.Member_Nr) + `
					WHERE uuid = "` + clubmember.UUID.String() + `"
				`
				println(sqlUpdateQuery)

				_, err4 := db.Exec(sqlUpdateQuery)
				if err4 != nil {
					c.JSON(400, err4.Error())
				} else {
					c.JSON(200, clubmember)
				}
			} else {
				c.JSON(500, errors.New("panic, more than 1 club member with same uuid: "+myUuid.String()))
			}
		}
	} else {
		c.JSON(400, errors.New("uuid is not valid"+clubmember.UUID.String()))
	}
}

func deleteDTOClubMember(c *gin.Context) {
	clubmem_uuid := c.Param("clubmem_uuid")
	deleteSQLStr := "delete from mitgliedschaft where uuid = ?"
	deleteDTOGeneric(c, clubmem_uuid, deleteSQLStr)
}

// table funktion
func getDTOClubOfficial(c *gin.Context) {
	official_uuid := c.Param("official_uuid")
	var clubofficial DTOClubOfficial

	if isValidUUID(official_uuid) {
		myUuid, _ := uuid.Parse(official_uuid)
		clubofficial.UUID = myUuid

		// TODO: we do not have and do not need member-uuid?
		var sqlSelectQuery string = `
			SELECT  
				ifnull(organisation.uuid, ''),
				ifnull(person.uuid, ''),
				ifnull(funktion.funktionsalias, ''),
				ifnull(funktion.von, ''),
				ifnull(funktion.bis, '')
			FROM funktion, 
				organisation,
				person
			WHERE funktion.organisation = organisation.id AND 
				funktion.person = person.id AND
				funktion.uuid = "` + official_uuid + `"`
		fmt.Println(sqlSelectQuery)

		var tmpClubUuid string
		var tmpPersonUuid string
		var validFrom string
		var validUntil string

		err := db.QueryRow(sqlSelectQuery).
			Scan(
				&tmpClubUuid,
				&tmpPersonUuid,
				&clubofficial.Role_Name,
				&validFrom,
				&validUntil,
			)

		var parseErrClubUUID error
		clubofficial.Club_UUID, parseErrClubUUID = uuid.Parse(tmpClubUuid)
		if parseErrClubUUID != nil {
			c.JSON(500, parseErrClubUUID.Error())
			return
		}
		var parseErrPersonUUID error
		clubofficial.Person_UUID, parseErrPersonUUID = uuid.Parse(tmpPersonUuid)
		if parseErrPersonUUID != nil {
			c.JSON(500, parseErrPersonUUID.Error())
			return
		}

		fmt.Println(validFrom)
		fmt.Println(validUntil)
		const layoutISO = "2006-01-02"
		if strings.Compare(validFrom, "") != 0 {
			tValidFrom, parseBDError := time.Parse(layoutISO, validFrom)
			if parseBDError != nil {
				c.JSON(500, parseBDError.Error())
				return
			} else {
				clubofficial.Valid_From = CivilTime(tValidFrom)
			}
		}
		if strings.Compare(validUntil, "") != 0 {
			tValidUntil, parseBDError2 := time.Parse(layoutISO, validUntil)
			if parseBDError2 != nil {
				c.JSON(500, parseBDError2.Error())
				return
			} else {
				clubofficial.Valid_Until = CivilTime(tValidUntil)
			}
		}

		if err != nil {
			c.JSON(500, err.Error())
			return
		}

		c.JSON(200, clubofficial)

	} else {
		c.JSON(400, errors.New("uuid is not valid "+official_uuid))
	}
}

func putDTOClubOfficial(c *gin.Context) {
	official_uuid := c.Param("official_uuid")
	var clubofficial DTOClubOfficial

	err := c.BindJSON(&clubofficial)
	if err != nil {
		c.JSON(400, err)
		return
	}

	if strings.Compare(official_uuid, clubofficial.UUID.String()) != 0 {
		c.JSON(400, errors.New("uuid from URL and uuid as JSON in body does not fits: "+official_uuid+" vs "+clubofficial.UUID.String()))
		return
	}

	if isValidUUID(clubofficial.UUID.String()) {
		myUuid, parseErr := uuid.Parse(clubofficial.UUID.String())

		if parseErr != nil {
			c.JSON(400, parseErr)
			return
		}

		var count string
		var sqlSelectQuery string = `select count(*) from funktion where uuid = "` + myUuid.String() + `"`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		fmt.Printf(sqlSelectQuery)

		if errDBExec != nil {
			c.JSON(500, err.Error())
		} else {

			var person_id, errPersonId = getIDFromUUID("person", clubofficial.Person_UUID)
			if errPersonId != nil {
				c.JSON(400, errPersonId.Error())
				return
			}
			var organisation_id, errOrganisationId = getIDFromUUID("organisation", clubofficial.Club_UUID)
			if errOrganisationId != nil {
				c.JSON(400, errOrganisationId.Error())
				return
			}
			var fromStr = time.Time(clubofficial.Valid_From).Format("2006-01-02")
			var untilStr = time.Time(clubofficial.Valid_Until).Format("2006-01-02")

			if strings.Compare(count, "0") == 0 { // insert

				// TODO add funktion as id as well
				var sqlInsertQuery string = `
					INSERT INTO funktion (
						uuid,
						organisation,
						person,
						funktionsalias,
						von,
						bis)
					VALUES ("` + clubofficial.UUID.String() +
					`", ` + strconv.Itoa(organisation_id) +
					`,` + strconv.Itoa(person_id) +
					`, "` + clubofficial.Role_Name +
					`", "` + fromStr +
					`", "` + untilStr + `")
				`
				println(sqlInsertQuery)

				_, err3 := db.Exec(sqlInsertQuery)

				if err3 != nil {
					c.JSON(400, err3.Error())
				} else {
					c.JSON(200, clubofficial)
				}
			} else if strings.Compare(count, "1") == 0 { // update

				var sqlUpdateQuery string = `
					UPDATE funktion set 
						organisation = ` + strconv.Itoa(organisation_id) + `,
						person = ` + strconv.Itoa(person_id) + `,
						funktionsalias = "` + clubofficial.Role_Name + `",
						von = "` + fromStr + `",
						bis = "` + untilStr + `"
					WHERE uuid = "` + clubofficial.UUID.String() + `"
				`
				println(sqlUpdateQuery)

				_, err4 := db.Exec(sqlUpdateQuery)
				if err4 != nil {
					c.JSON(400, err4.Error())
				} else {
					c.JSON(200, clubofficial)
				}
			} else {
				c.JSON(500, errors.New("panic, more than 1 club offical with same uuid: "+myUuid.String()))
			}
		}
	} else {
		c.JSON(400, errors.New("uuid is not valid"+clubofficial.UUID.String()))
	}
}

func deleteDTOClubOfficial(c *gin.Context) {
	official_uuid := c.Param("official_uuid")
	deleteSQLStr := "delete from funktion where uuid = ?"
	deleteDTOGeneric(c, official_uuid, deleteSQLStr)
}

func putDTORegion(c *gin.Context) {
	c.JSON(204, "Mivis does not support Region, so ignore and no handling of input.")
}

func getDTOPlayerLicense(c *gin.Context) {
	c.JSON(204, "Not implemented yet")
}

func putDTOPlayerLicense(c *gin.Context) {
	c.JSON(204, "Not implemented yet")
}

func deleteDTOPlayerLicense(c *gin.Context) {
	c.JSON(204, "Not implemented yet")
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

	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(LoggingMiddleware())

	public := router.Group("/public")

	public.POST("/register", registerLoginUser)
	public.POST("/login", loginUser)

	/*
		authorized := router.Group("/api", gin.BasicAuth(gin.Accounts{
			basicAuthUsername: basicAuthPassword,
		}))
	*/

	authorized := router.Group("/api")

	authorized.Use(jwtAuthMiddleware())

	authorized.PUT("/regions/:reg_uuid", putDTORegion)

	authorized.GET("/federations/:fed_uuid", getDTOFederation)
	authorized.PUT("/federations/:fed_uuid", putDTOFederation)

	authorized.GET("/clubs/:club_uuid", getDTOClub)
	authorized.PUT("/clubs/:club_uuid", putDTOClub)
	authorized.DELETE("/clubs/:club_uuid", deleteDTOClub)

	authorized.GET("/addresses/:addr_uuid", getDTOAddress)
	authorized.PUT("/addresses/:addr_uuid", putDTOAddress)
	authorized.DELETE("/addresses/:addr_uuid", deleteDTOAddress)

	authorized.GET("/persons/:pers_uuid", getDTOPerson)
	authorized.PUT("/persons/:pers_uuid", putDTOPerson)
	authorized.DELETE("/persons/:pers_uuid", deleteDTOPerson)

	authorized.GET("/club-members/:clubmem_uuid", getDTOClubMember)
	authorized.PUT("/club-members/:clubmem_uuid", putDTOClubMember)
	authorized.DELETE("/club-members/:clubmem_uuid", deleteDTOClubMember)

	authorized.GET("/club-officials/:official_uuid", getDTOClubOfficial)
	authorized.PUT("/club-officials/:official_uuid", putDTOClubOfficial)
	authorized.DELETE("/club-officials/:official_uuid", deleteDTOClubOfficial)

	authorized.GET("/player-licences/:license_uuid", getDTOPlayerLicense)
	authorized.PUT("/player-licences/:license_uuid", putDTOPlayerLicense)
	authorized.DELETE("/player-licences/:license_uuid", deleteDTOPlayerLicense)

	router.Run(":3030")
}
