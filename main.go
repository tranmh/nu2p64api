package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/natefinch/lumberjack"
	log "github.com/sirupsen/logrus"
//	"golang.org/x/text/encoding/charmap"
)

var (
	db                        *sql.DB
	yourMySQLdatabasepassword string
	basicAuthUsername         string
	basicAuthPassword         string
	errLog                    *log.Logger
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

		reqBody, _ := io.ReadAll(ctx.Request.Body)
		ctx.Request.Body = io.NopCloser(bytes.NewReader(reqBody))

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
	// log.SetFormatter(&log.JSONFormatter{})
}

// -----------------------------------------------------------------------------
// https://stackoverflow.com/questions/28796021/how-can-i-log-in-golang-to-a-file-with-log-rotation
func SetupLogger() {

	lumberjackLogger := &lumberjack.Logger{
		// Log file abbsolute path, os agnostic
		Filename:   filepath.ToSlash("main.log"),
		MaxSize:    500, // MB
		MaxBackups: 100,
		MaxAge:     30,   // days
		Compress:   true, // disabled by default
	}

	// Fork writing into two outputs
	multiWriter := io.MultiWriter(os.Stderr, lumberjackLogger)

	logFormatter := new(log.TextFormatter)
	logFormatter.TimestampFormat = time.RFC1123Z // or RFC3339
	logFormatter.FullTimestamp = true

	log.SetFormatter(logFormatter)
	log.SetLevel(log.InfoLevel)
	log.SetOutput(multiWriter)
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
		AbortWithStatusJSON(c, http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	CJSON(c, http.StatusOK, gin.H{"message": "validated!"})

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
		AbortWithStatusJSON(c, http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, err := loginCheck(input.Username, input.Password)

	if err != nil {
		AbortWithStatusJSON(c, http.StatusBadRequest, gin.H{"error": "username or password is incorrect."})
		return
	}

	CJSON(c, http.StatusOK, gin.H{"token": token})

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

// https://go.dev/play/p/Pn2qwpiRC4
// -----------------------------------------------------------------------------
func verifyTokenController() gin.HandlerFunc {
	return func(c *gin.Context) {
		prefix := "Bearer "
		authHeader := c.Request.Header.Get("Authorization")
		reqToken := strings.TrimPrefix(authHeader, prefix)

		if authHeader == "" || reqToken == authHeader {
			AbortWithStatusJSON(c, http.StatusUnauthorized, gin.H{"Message": "Authentication header not present or malformed"})
			return
		}

		envFile, _ := godotenv.Read(".env")

		if strings.Compare(reqToken, envFile["SECRET_TOKEN"]) != 0 {
			AbortWithStatusJSON(c, http.StatusUnauthorized, gin.H{"Message": "Authentication token is not correct"})
			return
		}
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

type Sex int

const (
	Female Sex = iota
	Male
	SexUnknown
)

func (s Sex) String() string {
	switch s {
	case Male:
		return "male"
	case Female:
		return "female"
	}
	return "sexunknown"
}

func getSex(sex string) Sex {
	if strings.Compare(sex, "female") == 0 {
		return Female
	} else if strings.Compare(sex, "male") == 0 {
		return Male
	} else {
		return SexUnknown
	}
}

type LicenseState int

const (
	LicenseStateUnknown LicenseState = 0
	LicenseStateActive  LicenseState = 1
	LicenseStatePassive LicenseState = 2
)

func LicenseStateToString(licState LicenseState) string {
	if licState == LicenseStateActive {
		return "ACTIVE"
	} else if licState == LicenseStatePassive {
		return "PASSIVE"
	} else {
		return "UNKNOWN"
	}
}

func getLicenseStateFromString(licStateStr string) LicenseState {
	if strings.Compare(strings.ToUpper(licStateStr), "ACTIVE") == 0 {
		return LicenseStateActive
	} else if strings.Compare(strings.ToUpper(licStateStr), "PASSIVE") == 0 {
		return LicenseStatePassive
	} else {
		return LicenseStateUnknown
	}
}

type istAbteilung int

const (
	Einsparten           istAbteilung = 0
	Mehrsparten          istAbteilung = 1
	UNKNOWN_istAbteilung istAbteilung = 2
)

func istAbteilungToClubType(ia istAbteilung) string {
	if ia == Einsparten {
		return "SINGLEDEVISION"
	} else if ia == Mehrsparten {
		return "MULTIDIVISION"
	} else {
		return "UNKNOWN_CLUBTYPE"
	}
}

func ClubTypeStringToistAbteilung(ct string) string {
	if strings.Compare(strings.ToUpper(ct), "SINGLEDEVISION") == 0 {
		return strconv.Itoa(int(Einsparten))
	} else if strings.Compare(strings.ToUpper(ct), "MULTIDIVISION") == 0 {
		return strconv.Itoa(int(Mehrsparten))
	} else {
		return strconv.Itoa(int(UNKNOWN_istAbteilung))
	}
}

func ClubRoleNameToFunktion(crm string) string {
	var id string
	var sqlSelectQuery string = `select id from funktionsart where bezeichnung = '` + EscapeTick(crm) + `'`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&id)
		//log.Info(sqlSelectQuery)

		if errDBExec != nil {
			return "0"
		} else {
			return id
		}
}

// -----------------------------------------------------------------------------

func ReplaceSpecialCharacters(myString string) string {
	// neue Konvertierung ueber charmap
	//out := make([]byte, 0)
	//var retString string

	//for _, r := range myString {
	//	if e, ok := charmap.ISO8859_1.EncodeRune(r); ok {
	//		//out = append(out, e)
	//		retString = retString + fmt.Sprintf("%c", e)
	//	}
	//}
	//myString = retString
	// alte Konvertierungen

	//myString = strings.ReplaceAll(myString, "\xC2", "")
	//myString = strings.ReplaceAll(myString, "\x84", "")
	//myString = strings.ReplaceAll(myString, "\u0084", "")
	//myString = strings.ReplaceAll(myString, "\x9E", "")
	//myString = strings.ReplaceAll(myString, "\u009e", "")

	// weitere Konvertierungen
	//myString = strings.ReplaceAll(myString, "\x96", "")
	//myString = strings.ReplaceAll(myString, "\x9A", "")
	//myString = strings.ReplaceAll(myString, "\x9F", "")
	//myString = strings.ReplaceAll(myString, "\xC3", "ß")
	
	// neue Konvertierungen
	myString = strings.ReplaceAll(myString, "\xC2\x96", " ")
	myString = strings.ReplaceAll(myString, "\u009a", "")
	myString = strings.ReplaceAll(myString, "\xc2\x9e", "s")
	
	return myString
}

func EscapeTick(input string) string {
	input = ReplaceSpecialCharacters(input)
	return strings.ReplaceAll(input, "'", "\\'")
}

func AbortWithStatusJSON(c *gin.Context, code int, jsonObj any) {
	fmt.Printf("%s ERROR code:%d jsonObj:%v \n", time.Now().Format(time.RFC3339), code, jsonObj)
	c.AbortWithStatusJSON(code, jsonObj)
}

func CJSON(c *gin.Context, code int, jsonObj any) {
	fmt.Printf("%s PASS code:%d jsonObj:%v \n", time.Now().Format(time.RFC3339), code, jsonObj)
	c.JSON(code, jsonObj)
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
	Region_UUID                 uuid.UUID   `json:"region-uuid"` // region-uuid is not in use in portal64, but we use verband, unterverband, bezirk, verein.
	Club_NR                     string      `json:"club-nr"`
	Name                        string      `json:"name"`
	Entry_Date                  CivilTime   `json:"entry-date"`
	Contact_Address_UUID        uuid.UUID   `json:"contact-address-uuid"`        // TODO table adresse
	Sport_Address_UUIDs         []uuid.UUID `json:"sport-address-uuids"`         // TODO table adressen
	Register_Of_Associations_Nr string      `json:"register-of-associations-nr"` // vereinsregister-Nr: no column in mivis, so ignore
	Club_Type                   string      `json:"club-type"`                   // =istAbteilung in mivis
	Bank_Account_Owner          string      `json:"bank-account-owner"`          // no column in mivis, so ignore
	Bank_Account_Bank           string      `json:"bank-account-bank"`           // no column in mivis, so ignore
	Bank_Account_BIC            string      `json:"bank-account-big"`            // no column in mivis, so ignore
	Bank_Account_IBAN           string      `json:"bank-account-iban"`           // no column in mivis, so ignore
	Direct_Debit                bool        `json:"direct-debit"`                // no column in mivis, so ignore
}

type DTOClubMember struct {
	UUID                uuid.UUID `json:"uuid"`
	Club_UUID           uuid.UUID `json:"club-uuid"`
	Person_UUID         uuid.UUID `json:"person-uuid"`
	Member_From         CivilTime `json:"member-from"`
	Member_Until        CivilTime `json:"member-until"`
	License_State       string    `json:"licence-state"` // ACTIVE, PASSIVE, NO_LICENSE // stat1: 1 aktiv, 2 passiv
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
	UUID          uuid.UUID `json:"uuid"`
	Federation_NR string    `json:"federation-nr"`
	Name          string    `json:"name"`
	NickName      string    `json:"nickname"`
	Region_UUID   uuid.UUID `json:"region-uuid"`
}

type DTOPerson struct {
	UUID          uuid.UUID `json:"uuid"`
	FirstName     string    `json:"firstname"`
	LastName      string    `json:"lastname"`
	Title         string    `json:"title"`
	Gender        string    `json:"gender"` // male, female, nonbinary; no column in mivis, so ignore
	Sex           string    `json:"sex"`    // male, female
	AddressUUID   uuid.UUID `json:"address-uuid"`
	BirthDate     CivilTime `json:"birthdate"`
	BirthPlace    string    `json:"birthplace"`
	BirthName     string    `json:"birthname"` // TODO: no column in Mivis, so ignore
	Dead          int       `json:"dead"`
	Nation        string    `json:"nation"`
	Privacy_State bool      `json:"privacy-state"` // datenschutz: 1 = zugestimmt.
	Remarks       string    `json:"remarks"`
	FIDE_Title    string    `json:"fide-title"` // TODO: no column in Mivis, so ignore
	FIDE_Nation   string    `json:"fide-nation"`
	FIDE_Id       string    `json:"fide-id"`
}

func validateDTOAddress(dtoaddress DTOAddress) (bool, error) {
	var err error

	if strings.TrimSpace(dtoaddress.Email) != "" {
		_, err = mail.ParseAddress(dtoaddress.Email)
		if err != nil {
			return false, err
		}
	}

	if strings.TrimSpace(dtoaddress.Email2) != "" {
		_, err = mail.ParseAddress(dtoaddress.Email2)
		if err != nil {
			return false, err
		}
	}

	if strings.TrimSpace(dtoaddress.WWW) != "" {

		if !strings.HasPrefix(strings.ToLower(dtoaddress.WWW), "https://") &&
			!strings.HasPrefix(strings.ToLower(dtoaddress.WWW), "http://") {
			dtoaddress.WWW = "https://" + dtoaddress.WWW
		}

		_, err = url.ParseRequestURI(dtoaddress.WWW)
		if err != nil {
			return false, err
		}
	}

	// https://www.golangprograms.com/regular-expression-to-validate-phone-number.html
	re := regexp.MustCompile(`^(?:(?:\(?(?:00|\+)([1-4]\d\d|[1-9]\d?)\)?)?[\-\.\ \\\/]?)?((?:\(?\d{1,}\)?[\-\.\ \\\/]?){0,})(?:[\-\.\ \\\/]?(?:#|ext\.?|extension|x)[\-\.\ \\\/]?(\d+))?$`)

	if !re.MatchString(dtoaddress.Phone_Mobile) {
		return false, errors.New("dtoaddress.Phone_Mobile is not a valid phone number: " + dtoaddress.Phone_Mobile)
	}

	if !re.MatchString(dtoaddress.Phone_Home) {
		return false, errors.New("dtoaddress.Phone_Home is not a valid phone number: " + dtoaddress.Phone_Home)
	}

	if !re.MatchString(dtoaddress.Phone_Work) {
		return false, errors.New("dtoaddress.Phone_Work is not a valid phone number: " + dtoaddress.Phone_Work)
	}

	return true, nil
}

type DTORegion struct {
	UUID               uuid.UUID `json:"uuid"`
	Name               string    `json:"name"`
	NickName           string    `json:"nickname"`
	Pattern            string    `json:"pattern"`
	Parent_Region_UUID uuid.UUID `json:"parent-region-uuid"`
}

type DTOPlayerLicense struct {
	UUID              uuid.UUID `json:"uuid"`
	Club_UUID         uuid.UUID `json:"club-uuid"`
	Person_UUID       uuid.UUID `json:"person-uuid"`
	RequestDate       CivilTime `json:"request-date"`
	LicenseValidFrom  CivilTime `json:"licence-valid-from"`
	LicenseValidUntil CivilTime `json:"licence-valid-until"`
	License_State     string    `json:"licence-state"` // ACTIVE, PASSIVE
	Member_Nr         int       `json:"member-nr"`     // PKZ
}

// -----------------------------------------------------------------------------

func init() {
	initLog()
	SetupLogger()
}

func isValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func parseStringToCivilTime(input string) (CivilTime, error) {
	const layoutISO = "2006-01-02"
	t, parseError := time.Parse(layoutISO, input)
	return CivilTime(t), parseError
}

func CivilTimeToString(civilTime CivilTime) string {
	ret := time.Time(civilTime).Format("2006-01-02")
	if ret == "0001-01-01" {
		ret = ""
	}
	return ret
}

// select
func getDTOPerson(c *gin.Context) {
	uuidParam := c.Param("pers_uuid")

	var person DTOPerson
	// var tmpBirthDay string

	if isValidUUID(uuidParam) {

		var count string
		var sqlSelectQueryCount string = `select count(*) from person where uuid = '` + EscapeTick(uuidParam) + `'`
		errDBExec := db.QueryRow(sqlSelectQueryCount).Scan(&count)
		log.Info(sqlSelectQueryCount)

		if errDBExec != nil {
			AbortWithStatusJSON(c, 500, errDBExec.Error())
			return
		} else {
			if strings.Compare(count, "0") == 0 {
				AbortWithStatusJSON(c, 404, "person with following uuid was not found in database: "+uuidParam)
				return
			}
		}

		myUuid, _ := uuid.Parse(uuidParam)
		person.UUID = myUuid
		var tmpAdresseID int
		var tmpBirthDate string
		var tmpPrivacyState string

		sqlSelectQuery := `
		select ifnull(person.name, ''), 
				ifnull(person.vorname, ''), 
				ifnull(titel.bezeichnung, ''), 
				ifnull(person.geschlecht, ''), 
				ifnull(person.adress, ''), 
				ifnull(person.geburtsdatum, ''), 
				ifnull(person.geburtsort, ''), 
				ifnull(person.verstorben, ''), 
				ifnull(person.nation, ''), 
				ifnull(person.datenschutz, ''), 
				ifnull(person.bemerkung, ''), 
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
				&person.Sex,
				&tmpAdresseID,
				&tmpBirthDate,
				&person.BirthPlace,
				&person.Dead,
				&person.Nation,
				&tmpPrivacyState, // TODO: NULL, 0 or 1, 1 means accepted?
				&person.Remarks,
				// we do not have FIDE_Title, so ignore
				&person.FIDE_Nation,
				&person.FIDE_Id,
			)

		if strings.Compare(person.Sex, "0") == 0 {
			person.Sex = "female"
		} else if strings.Compare(person.Sex, "1") == 0 {
			person.Sex = "male"
		} else {
			AbortWithStatusJSON(c, 500, errors.New("neither female=0 nor male=1 - broken data with sex aka person.geschlecht column? sex:"+person.Sex))
			return
		}

		if strings.Compare(tmpPrivacyState, "0") == 0 {
			person.Privacy_State = false
		} else if strings.Compare(tmpPrivacyState, "1") == 0 {
			person.Privacy_State = false
		} else {
			AbortWithStatusJSON(c, 500, errors.New("neither privacy_state=0 nor privacy_state=1 - broken data with privcay_state aka column person.datenschutz: "+tmpPrivacyState))
			return
		}

		var parseBDError error
		person.BirthDate, parseBDError = parseStringToCivilTime(tmpBirthDate)
		if parseBDError != nil {
			AbortWithStatusJSON(c, 500, parseBDError.Error())
			return
		}

		var errGetUUIDFromID error
		person.AddressUUID, errGetUUIDFromID = getUUIDFromID("adresse", tmpAdresseID)

		if errGetUUIDFromID != nil {
			AbortWithStatusJSON(c, 500, errGetUUIDFromID.Error())
			return
		}

		if err != nil {
			AbortWithStatusJSON(c, 500, err.Error())
			return
		} else {
			CJSON(c, 200, person)
			return
		}
	} else {
		AbortWithStatusJSON(c, 400, "uuidParam is not valid: "+uuidParam)
		return
	}
}

func deleteDTOGeneric(c *gin.Context, uuidParam string, deleteSQLStr string) {
	if isValidUUID(uuidParam) {
		myUuid, _ := uuid.Parse(uuidParam)

		result, err := db.Exec(deleteSQLStr, myUuid)

		if err != nil {
			AbortWithStatusJSON(c, 500, err.Error())
			return
		} else {
			rowsAffected, err2 := result.RowsAffected()
			if err2 != nil {
				AbortWithStatusJSON(c, 500, err2.Error())
				return
			} else if rowsAffected == 0 {
				AbortWithStatusJSON(c, 404, rowsAffected)
				return
			} else if rowsAffected == 1 {
				CJSON(c, 200, rowsAffected)
				return
			} else {
				AbortWithStatusJSON(c, 500, rowsAffected)
				return
			}
		}
	} else {
		AbortWithStatusJSON(c, 400, "uuidParam is not valid: "+uuidParam)
		return
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
	pers_uuid := c.Param("pers_uuid")

	var person DTOPerson
	err := c.BindJSON(&person)
	if err != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}

	if strings.Compare(pers_uuid, person.UUID.String()) != 0 {
		AbortWithStatusJSON(c, 400, "uuid from URL and uuid as JSON in body does not fit: "+pers_uuid+" vs "+person.UUID.String())
		return
	}

	if isValidUUID(person.UUID.String()) {
		myUuid, parseErr := uuid.Parse(person.UUID.String())

		if parseErr != nil {
			AbortWithStatusJSON(c, 400, parseErr.Error())
			return
		}

		if !isValidUUID(myUuid.String()) {
			AbortWithStatusJSON(c, 400, errors.New("myUuid is not a valid UUID: "+myUuid.String()))
			return
		}

		var count string
		var sqlSelectQuery string = `select count(*) from person where uuid = '` + EscapeTick(myUuid.String()) + `'`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		log.Info(sqlSelectQuery)

		if errDBExec != nil {
			AbortWithStatusJSON(c, 500, errDBExec.Error())
			return
		} else {
			var title = convertTitleToTitleID(person.Title)
			var sex = getSex(person.Sex)
			var birthday = strconv.Itoa(time.Time(person.BirthDate).Year()) + "-" + strconv.Itoa(int(time.Time(person.BirthDate).Month())) + "-" + strconv.Itoa(time.Time(person.BirthDate).Day())
			var addressID, errGetIDFromUUID = getIDFromUUIDOrCreateDummyData("adresse", person.AddressUUID)
			if errGetIDFromUUID != nil {
				AbortWithStatusJSON(c, 400, errGetIDFromUUID.Error()+" UUID was not found in table adresse")
				return
			}
			if strings.Compare(person.FIDE_Id, "") == 0 {
				person.FIDE_Id = "NULL"
			}

			var privacy_state_int = 0
			if person.Privacy_State {
				privacy_state_int = 1
			}

			if strings.Compare(count, "0") == 0 { // insert

				var sqlInsertQuery string = `
					INSERT INTO person (
						uuid,
						name, 
						vorname, 
						titel, 
						geschlecht, 
						adress,
						geburtsdatum,
						geburtsort,
						verstorben, 
						nation, 
						datenschutz,
						bemerkung, 
						nationfide, 
						idfide)
					VALUES ('` + person.UUID.String() +
					`', '` + EscapeTick(person.LastName) +
					`', '` + EscapeTick(person.FirstName) +
					`', ` + strconv.Itoa(title) +
					`, ` + strconv.Itoa(int(sex)) +
					`, ` + strconv.Itoa(addressID) +
					`, '` + birthday +
					`', '` + EscapeTick(person.BirthPlace) +
					`', ` + strconv.Itoa(person.Dead) +
					`, '` + EscapeTick(person.Nation) +
					`', '` + strconv.Itoa(privacy_state_int) +
					`', '` + EscapeTick(person.Remarks) +
					`', '` + EscapeTick(person.FIDE_Nation) +
					`',` + EscapeTick(person.FIDE_Id) +
					`)
				`
				log.Info(sqlInsertQuery)

				_, err3 := db.Exec(sqlInsertQuery)

				if err3 != nil {
					AbortWithStatusJSON(c, 400, err3.Error())
					return
				} else {
					CJSON(c, 200, person)
					return
				}
			} else if strings.Compare(count, "1") == 0 { // update

				var sqlUpdateQuery string = `
					UPDATE person set 
						name = '` + EscapeTick(person.LastName) + `',
						vorname = '` + EscapeTick(person.FirstName) + `',
						titel = '` + strconv.Itoa(title) + `',
						geschlecht = '` + strconv.Itoa(int(sex)) + `',
						adress = '` + strconv.Itoa(addressID) + `',
						geburtsdatum = '` + birthday + `',
						geburtsort = '` + EscapeTick(person.BirthPlace) + `',
						verstorben = '` + strconv.Itoa(person.Dead) + `',
						nation = '` + EscapeTick(person.Nation) + `',
						datenschutz = '` + strconv.Itoa(privacy_state_int) + `',
						bemerkung = '` + EscapeTick(person.Remarks) + `',
						nationfide = '` + EscapeTick(person.FIDE_Nation) + `',
						idfide = ` + EscapeTick(person.FIDE_Id) + `
					WHERE uuid = '` + person.UUID.String() + `'
				`
				log.Infoln(sqlUpdateQuery)

				_, err4 := db.Exec(sqlUpdateQuery)
				if err4 != nil {
					AbortWithStatusJSON(c, 400, err4.Error())
					return
				} else {
					CJSON(c, 200, person)
					return
				}
			} else {
				AbortWithStatusJSON(c, 500, "panic, more than 1 federation with same uuid: "+myUuid.String())
				return
			}
		}
	} else {
		AbortWithStatusJSON(c, 400, errors.New("uuid is not valid"+person.UUID.String()))
		return
	}
}

// table organisation for verein und verband
func getDTOFederation(c *gin.Context) {
	fed_uuid := c.Param("fed_uuid")
	var federation DTOFederation

	if isValidUUID(fed_uuid) {

		myUuid, _ := uuid.Parse(fed_uuid)
		federation.UUID = myUuid

		var count string
		var sqlSelectQuery string = `select count(*) from organisation where uuid = '` + myUuid.String() + `'`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		log.Info(sqlSelectQuery)

		if errDBExec != nil {
			AbortWithStatusJSON(c, 500, errDBExec.Error())
			return
		} else {
			if strings.Compare(count, "0") == 0 {
				AbortWithStatusJSON(c, 404, "federation with the uuid was not found in the database: "+myUuid.String())
				return
			}
		}

		err := db.QueryRow("SELECT name, vkz, kurzname FROM `organisation` where uuid = ?", myUuid).
			Scan(
				&federation.Name,
				&federation.Federation_NR,
				&federation.NickName,
			)

		if err != nil {
			AbortWithStatusJSON(c, 500, err.Error())
			return
		} else {
			CJSON(c, 200, federation)
			return
		}
	} else {
		AbortWithStatusJSON(c, 400, "fed_uuid is not valid: "+fed_uuid)
		return
	}
}

func putDTOFederation(c *gin.Context) {
	fed_uuid := c.Param("fed_uuid")

	var federation DTOFederation
	err := c.BindJSON(&federation)
	if err != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}

	if strings.Compare(fed_uuid, federation.UUID.String()) != 0 {
		AbortWithStatusJSON(c, 400, "uuid from URL and uuid as JSON in body does not fit: "+fed_uuid+" vs "+federation.UUID.String())
		return
	}

	if isValidUUID(federation.UUID.String()) {
		myUuid, parseErr := uuid.Parse(federation.UUID.String())

		if parseErr != nil {
			AbortWithStatusJSON(c, 400, parseErr.Error())
			return
		}

		if !isValidUUID(myUuid.String()) {
			AbortWithStatusJSON(c, 400, errors.New("myUuid is not a valid UUID: "+myUuid.String()))
			return
		}

		var count string
		var sqlSelectQuery string = `select count(*) from organisation where uuid = '` + myUuid.String() + `'`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		log.Info(sqlSelectQuery)

		if errDBExec != nil {
			AbortWithStatusJSON(c, 500, errDBExec.Error())
			return
		} else {

			if strings.Compare(count, "0") == 0 { // insert

				var sqlInsertQuery string = `
					INSERT INTO organisation (
						uuid,
						name, 
						vkz,
						kurzname)
					VALUES ('` + federation.UUID.String() +
					`', '` + EscapeTick(federation.Name) +
					`', '` + EscapeTick(federation.Federation_NR) +
					`', '` + EscapeTick(federation.NickName) + `')
				`
				log.Infoln(sqlInsertQuery)

				_, err3 := db.Exec(sqlInsertQuery)

				if err3 != nil {
					AbortWithStatusJSON(c, 400, err3.Error())
					return
				} else {
					CJSON(c, 200, federation)
					return
				}
			} else if strings.Compare(count, "1") == 0 { // update

				var sqlUpdateQuery string = `
					UPDATE organisation set 
						name = '` + EscapeTick(federation.Name) + `',
--						vkz = '` + EscapeTick(federation.Federation_NR) + `',
						kurzname = '` + EscapeTick(federation.NickName) + `'
					WHERE uuid = '` + federation.UUID.String() + `'
				`
				log.Infoln(sqlUpdateQuery)

				_, err4 := db.Exec(sqlUpdateQuery)
				if err4 != nil {
					AbortWithStatusJSON(c, 400, err4.Error())
					return
				} else {
					CJSON(c, 200, federation)
					return
				}
			} else {
				AbortWithStatusJSON(c, 500, errors.New("panic, more than 1 federation with same uuid: "+myUuid.String()))
				return
			}
		}
	} else {
		AbortWithStatusJSON(c, 400, errors.New("uuid is not valid"+federation.UUID.String()))
		return
	}
}

func deleteDTOFederation(c *gin.Context) {
	fed_uuid := c.Param("fed_uuid")
	deleteSQLStr := "delete from organisation where uuid = ?"
	deleteDTOGeneric(c, fed_uuid, deleteSQLStr)
}

func getSportAddressUUIDs(organisationId int) ([]uuid.UUID, error) {

	var result []uuid.UUID
	var sqlSelectQuery string = `SELECT uuid FROM adressen WHERE typ = 5 AND organisation = ` + strconv.Itoa(organisationId)

	rows, err := db.Query(sqlSelectQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var addrUUID string
	for rows.Next() {
		err := rows.Scan(&addrUUID)
		if err != nil {
			return nil, err
		} else {
			var uuid, parseErr = uuid.Parse(addrUUID)
			if parseErr != nil {
				return nil, parseErr
			} else {
				result = append(result, uuid)
			}
		}
	}

	return result, nil
}

// table organisation as well
func getDTOClub(c *gin.Context) {
	club_uuid := c.Param("club_uuid")
	var club DTOClub

	if !isValidUUID(club_uuid) {
		AbortWithStatusJSON(c, 400, "club_uuid is not valid: "+club_uuid)
		return
	}

	myUuid, _ := uuid.Parse(club_uuid)
	club.UUID = myUuid

	var count string
	var sqlSelectQueryCount string = `select count(*) from organisation where uuid = '` + myUuid.String() + `'`
	errDBExec := db.QueryRow(sqlSelectQueryCount).Scan(&count)
	log.Info(sqlSelectQueryCount)

	if errDBExec != nil {
		AbortWithStatusJSON(c, 500, errDBExec.Error())
		return
	} else {
		if strings.Compare(count, "0") == 0 {
			AbortWithStatusJSON(c, 404, "club with the uuid was not found in the database: "+myUuid.String())
			return
		}
	}

	var tmpEntryDate string
	var tmpAddressId int
	var tmpOrganisationId int
	var tmpClubType int

	var sqlSelectQuery = `
		SELECT 
			ifnull(organisation.vkz, ''),
			ifnull(organisation.name, ''),
			ifnull(organisation.grundungsdatum, ''),
			ifnull(organisation.adress, '-1'),
			ifnull(organisation.id, ''),
			ifnull(organisation.istAbteilung, '')
		FROM organisation 
		WHERE uuid = ?
		`
	err := db.QueryRow(sqlSelectQuery, myUuid).
		Scan(
			&club.Club_NR,
			&club.Name,
			&tmpEntryDate,
			&tmpAddressId,
			&tmpOrganisationId,
			&tmpClubType,
		)
	if err != nil {
		AbortWithStatusJSON(c, 500, err.Error())
		return
	}

	if strings.Compare(tmpEntryDate, "") != 0 {
		var parseError error
		club.Entry_Date, parseError = parseStringToCivilTime(tmpEntryDate)
		if parseError != nil {
			AbortWithStatusJSON(c, 500, parseError.Error())
			return
		}
	}

	if tmpAddressId != -1 {
		club.Contact_Address_UUID, err = getUUIDFromID("adresse", tmpAddressId)
		if err != nil {
			AbortWithStatusJSON(c, 500, err.Error())
			return
		}
	}

	club.Sport_Address_UUIDs, err = getSportAddressUUIDs(tmpOrganisationId)
	if err != nil {
		AbortWithStatusJSON(c, 500, err.Error())
		return
	}
	club.Club_Type = istAbteilungToClubType(istAbteilung(tmpClubType))

	var vkzVerband string = string(club.Club_NR[0]) + "00"

	err2 := db.QueryRow("SELECT uuid from `organisation` where vkz = ?", vkzVerband).
		Scan(
			&club.Federation_UUID,
		)
	if err2 != nil {
		AbortWithStatusJSON(c, 500, err2.Error())
		return
	}

	CJSON(c, 200, club)
}

func putDTOClub(c *gin.Context) {
	club_uuid := c.Param("club_uuid")
	var club DTOClub
	err := c.BindJSON(&club)
	if err != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}

	if strings.Compare(club_uuid, club.UUID.String()) != 0 {
		AbortWithStatusJSON(c, 400, "uuid from URL and uuid as JSON in body does not fit: "+club_uuid+" vs "+club.UUID.String())
		return
	}

	if isValidUUID(club.UUID.String()) {
		myUuid, parseErr := uuid.Parse(club.UUID.String())

		if parseErr != nil {
			AbortWithStatusJSON(c, 400, parseErr.Error())
			return
		}

		if !isValidUUID(myUuid.String()) {
			AbortWithStatusJSON(c, 400, errors.New("myUuid is not a valid UUID: "+myUuid.String()))
			return
		}

		var count string
		var sqlSelectQuery string = `select count(*) from organisation where uuid = '` + myUuid.String() + `'`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		log.Info(sqlSelectQuery)

		if errDBExec != nil {
			AbortWithStatusJSON(c, 500, err.Error())
			return
		} else {

			var gruendungsdatum string = CivilTimeToString(club.Entry_Date)
			var addressID, errGetIDFromUUID = getIDFromUUIDOrCreateDummyData("adresse", club.Contact_Address_UUID)
			if errGetIDFromUUID != nil {
				AbortWithStatusJSON(c, 400, errGetIDFromUUID.Error()+" UUID was not found in table adresse")
				return
			}
			if strings.Compare(count, "0") == 0 { // insert

				// TODO, extend this please with missing attributes => erledigt
				var sqlInsertQuery string
				if len(club.Club_NR) >= 5 {
					if gruendungsdatum == "" {
						sqlInsertQuery = `
							INSERT INTO organisation (
								uuid,
								name, 
								adress,
								adristperson,
								vkz,
								verband,
								unterverband,
								bezirk,
								verein,
								istAbteilung)
							VALUES ('` + club.UUID.String() +
							`', '` + EscapeTick(club.Name) +
							`', `  + strconv.Itoa(addressID) +
							`, `   + strconv.Itoa(0) +
							`, '`  + EscapeTick(club.Club_NR[0:5]) +
							`', '` + EscapeTick(string(club.Club_NR[0])) +
							`', '` + EscapeTick(string(club.Club_NR[1])) +
							`', '` + EscapeTick(string(club.Club_NR[2])) +
							`', '` + EscapeTick(club.Club_NR[3:5]) +
							`', `  + ClubTypeStringToistAbteilung(club.Club_Type) + `)
						`
					} else {
						sqlInsertQuery = `
							INSERT INTO organisation (
								uuid,
								name, 
								adress,
								adristperson,
								vkz,
								verband,
								unterverband,
								bezirk,
								verein,
								grundungsdatum,
								istAbteilung)
							VALUES ('` + club.UUID.String() +
							`', '` + EscapeTick(club.Name) +
							`', `  + strconv.Itoa(addressID) +
							`, `   + strconv.Itoa(0) +
							`, '`  + EscapeTick(club.Club_NR[0:5]) +
							`', '` + EscapeTick(string(club.Club_NR[0])) +
							`', '` + EscapeTick(string(club.Club_NR[1])) +
							`', '` + EscapeTick(string(club.Club_NR[2])) +
							`', '` + EscapeTick(club.Club_NR[3:5]) +
							`', '` + gruendungsdatum +
							`', `  + ClubTypeStringToistAbteilung(club.Club_Type) + `)
						`
					}
					log.Infoln(sqlInsertQuery)

					_, err3 := db.Exec(sqlInsertQuery)

					if err3 != nil {
						AbortWithStatusJSON(c, 400, err3.Error())
						return
					} else {
						CJSON(c, 200, club)
						return
					}
				} else {
					CJSON(c, 200, club)
					return
				}
			} else if strings.Compare(count, "1") == 0 { // update

				// TODO, extend this please with missing attributes => erledigt
				var sqlUpdateQuery string
				if len(club.Club_NR) >= 5 {
					if gruendungsdatum == "" {
						sqlUpdateQuery = `
							UPDATE organisation SET 
								name = '` + EscapeTick(club.Name) + `',
								adress = '` + strconv.Itoa(addressID) + `',
								adristperson = 0` + `,
								vkz = '` + EscapeTick(club.Club_NR[0:5]) + `',
								verband = '` + EscapeTick(string(club.Club_NR[0])) + `',
								unterverband = '` + EscapeTick(string(club.Club_NR[1])) + `',
								bezirk = '` + EscapeTick(string(club.Club_NR[2])) + `',
								verein = '` + EscapeTick(club.Club_NR[3:5]) + `',
								istAbteilung = '` + ClubTypeStringToistAbteilung(club.Club_Type) + `'
							WHERE uuid = '` + club.UUID.String() + `'
						`
					} else {
						sqlUpdateQuery = `
							UPDATE organisation SET 
								name = '` + EscapeTick(club.Name) + `',
								adress = '` + strconv.Itoa(addressID) + `',
								adristperson = 0` + `,
								vkz = '` + EscapeTick(club.Club_NR[0:5]) + `',
								verband = '` + EscapeTick(string(club.Club_NR[0])) + `',
								unterverband = '` + EscapeTick(string(club.Club_NR[1])) + `',
								bezirk = '` + EscapeTick(string(club.Club_NR[2])) + `',
								verein = '` + EscapeTick(club.Club_NR[3:5]) + `',
								grundungsdatum = '` + gruendungsdatum + `',	
								istAbteilung = '` + ClubTypeStringToistAbteilung(club.Club_Type) + `'
							WHERE uuid = '` + club.UUID.String() + `'
						`
					}
					log.Infoln(sqlUpdateQuery)

					_, err4 := db.Exec(sqlUpdateQuery)
					if err4 != nil {
						AbortWithStatusJSON(c, 400, err4.Error())
						return
					} else {
						CJSON(c, 200, club)
						return
					}
				} else {
					CJSON(c, 200, club)
					return
				}
			} else {
				AbortWithStatusJSON(c, 500, errors.New("panic, more than 1 club with same uuid: "+myUuid.String()))
				return
			}
			/*
			-- mitgliedschaftOrganisation speichern
			var idOrganisation string
			sqlSelectQuery string = `select id from organisation where uuid = '` + myUuid.String() + `'`
			errDBExec = db.QueryRow(sqlSelectQuery).Scan(&idOrganisation)
			log.Info(sqlSelectQuery)

			if errDBExec != nil {
				AbortWithStatusJSON(c, 500, err.Error())
				return
			} else {
			
				-- passender Bezirk suchen
				sqlSelectQuery = `select id from organisation where 
								  verband = '` + EscapeTick(string(club.Club_NR[0])) + `' and
								  unterverband = '` + EscapeTick(string(club.Club_NR[1])) + `' and
								  bezirk = '` + EscapeTick(string(club.Club_NR[2])) + `' and
								  organisationsart = 40
								 `
			var idParent string
			errDBExec = db.QueryRow(sqlSelectQuery).Scan(&idParent)
			log.Info(sqlSelectQuery)

			if errDBExec != nil {
				log.error(err.Error())
				-- Unterverband suchen
				return
			} else {
				-- Bezirk gefunden, mitgliedschaftOrganisation suchen
			    var idMitgliedschaftOrganisation string
				sqlSelectQuery = `select id from mitgliedschaftOrganisation where
								  organisation = '` + idOrganisation + `' and
								  parent = '` + idParent + `'
				errDBExec = db.QueryRow(sqlSelectQuery).Scan(&idMitgliedschaftOrganisation)
				log.Info(sqlSelectQuery)
				if errDBExec != nil {
				    -- mitgliedschaftOrganisation neu anlegen
					var sqlInsertQuery string
					sqlInsertQuery = `insert into mitgliedschaftOrganisation (
										organisation,
										idParent,
										von)
									  values (
										'` + idOrganisation +
										`', '` + idParent +
										`', CURRENT_DATE() )
									 `
					log.Infoln(sqlInsertQuery)

					_, err5 := db.Exec(sqlInsertQuery)

					if err3 != nil {
						log.Error(err5.Error())
					}
				}
			}
			*/
		}
	} else {
		AbortWithStatusJSON(c, 400, errors.New("uuid is not valid"+club.UUID.String()))
		return
	}
}

func deleteDTOClub(c *gin.Context) {
	club_uuid := c.Param("club_uuid")
	deleteSQLStr := "delete from organisation where uuid = ?"
	deleteDTOGeneric(c, club_uuid, deleteSQLStr)
}

func isValidUUIDofTable(myUuid string, tableName string) bool {
	if !isValidUUID(myUuid) {
		return false
	}

	var count string
	var sqlQuerySelect = "SELECT COUNT(*) from " + tableName + " WHERE uuid like '" + myUuid + "'"
	errDBExec := db.QueryRow(sqlQuerySelect).Scan(&count)
	log.Info(sqlQuerySelect)

	if errDBExec != nil {
		log.Errorln(errDBExec.Error())
		return false
	} else {
		if strings.Compare(count, "1") == 0 {
			return true
		} else {
			return false
		}
	}
}

func getDTOAddress(c *gin.Context) {
	addr_uuid := c.Param("addr_uuid")

	if isValidUUIDofTable(addr_uuid, "adressen") {
		getDTOAddressFromTableAdressen(c)
	} else if isValidUUIDofTable(addr_uuid, "adresse") {
		getDTOAddressFromTableAdresse(c)
	} else {
		AbortWithStatusJSON(c, 404, "addr_uuid: "+addr_uuid+" was neither found in table adresse nor table adressen")
		return
	}
}

func getDTOAddressFromTableAdressen(c *gin.Context) {
	addr_uuid := c.Param("addr_uuid")
	var address DTOAddress

	address.UUID, _ = uuid.Parse(addr_uuid)

	var sqlQuerySelect = `
			SELECT 
				ifnull(adr.wert, ''),
				ifnull(adr_art.id, '')
			FROM adressen, adr, adr_art
			WHERE
				adressen.id = adr.id_adressen AND
				adr_art.id = adr.id_art AND
				adressen.uuid = '` + addr_uuid + "'"

	log.Infoln(sqlQuerySelect)

	rows, err := db.Query(sqlQuerySelect)
	if err != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}
	defer rows.Close()

	var addrValue string
	var addrId int
	for rows.Next() {
		err := rows.Scan(&addrValue, &addrId)
		if err != nil {
			AbortWithStatusJSON(c, 401, err.Error())
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
				AbortWithStatusJSON(c, 500, err.Error())
				return
			}
		}
		if addrId == 18 {
			address.Longitude, err = strconv.ParseFloat(addrValue, 64)
			if err != nil {
				AbortWithStatusJSON(c, 500, err.Error())
				return
			}
		}
	}
	if err := rows.Err(); err != nil {
		AbortWithStatusJSON(c, 402, err)
		return
	}

	CJSON(c, 200, address)
}

func getDTOAddressFromTableAdresse(c *gin.Context) {
	addr_uuid := c.Param("addr_uuid")
	var address DTOAddress

	myUuid, _ := uuid.Parse(addr_uuid)
	address.UUID = myUuid

	var sqlSelectQuery string = `
	SELECT
		ifnull(land.bezeichnung, '') as land,
		ifnull(adresse.plz, '') as plz,
		ifnull(adresse.ort, '') as ort,
		ifnull(adresse.strasse, '') as strasse,
		ifnull(adresse.tel1, '') as tel1,
		ifnull(adresse.tel2, '') as tel2,
		ifnull(adresse.tel3, '') as tel3,
		ifnull(adresse.email1, '') as email1,
		ifnull(adresse.email2, '') as email2
	FROM adresse, land
	WHERE adresse.land = land.id AND uuid like '` + addr_uuid + `'
	`
	log.Infoln(sqlSelectQuery)

	err := db.QueryRow(sqlSelectQuery).
		Scan(
			&address.Country,
			&address.ZIP,
			&address.City,
			&address.Street,
			&address.Phone_Home,
			&address.Phone_Work,
			&address.Phone_Mobile,
			&address.Email,
			&address.Email2,
		)

	if err != nil {
		AbortWithStatusJSON(c, 500, err.Error())
		return
	} else {
		CJSON(c, 200, address)
	}
}

func updateAdrTableWithValue(addrValue string, id_address int, id_art int, c *gin.Context) {
	var sqlUpdateQuery = `
	UPDATE adr set 
		wert = '` + EscapeTick(addrValue) + `'
	WHERE id_adressen = ` + strconv.Itoa(id_address) + ` AND
		id_art = ` + strconv.Itoa(id_art)
	log.Infoln(sqlUpdateQuery)
	_, err := db.Exec(sqlUpdateQuery)
	if err != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}
}

func putDTOAddress(c *gin.Context) {
	addr_uuid := c.Param("addr_uuid")

	// update or insert?
	// table adresse or adressen?
	if isValidUUIDofTable(addr_uuid, "adressen") {
		updateDTOAddressOnTableAdressen(c)
	} else if isValidUUIDofTable(addr_uuid, "adresse") {
		updateDTOAddressOnTableAdresse(c)
	} else {
		insertDTOAddressIntoTableAdresse(c) // TODO: no support of inserting to table adressen yet!
	}
}

func updateDTOAddressOnTableAdressen(c *gin.Context) {
	addr_uuid := c.Param("addr_uuid")
	var addressOfClub DTOAddress

	err := c.BindJSON(&addressOfClub)
	if err != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}

	validateDTOAddress, _ := validateDTOAddress(addressOfClub)

	if strings.Compare(addr_uuid, addressOfClub.UUID.String()) != 0 {
		AbortWithStatusJSON(c, 400, "uuid from URL and uuid as JSON in body does not fit: "+addr_uuid+" vs "+addressOfClub.UUID.String())
		return
	}

	var tmpIdAddress, _ = getIDFromUUIDOrCreateDummyData("adressen", addressOfClub.UUID)

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

	if validateDTOAddress {
		CJSON(c, 200, addressOfClub)
	} else {
		CJSON(c, 202, addressOfClub)
	}
}

func getCountryIdByNameAKABezeichnung(countryNameAKABezeichnung string) (result int, err error) {
	if (countryNameAKABezeichnung == "NULL") || (countryNameAKABezeichnung == "? unbekannt ?") || (countryNameAKABezeichnung == "?") || (countryNameAKABezeichnung == "") {
		return 1, nil
	}

	var selectQueryStr = "SELECT id FROM land where bezeichnung like '" + countryNameAKABezeichnung + "' OR ioc like '" + countryNameAKABezeichnung + "'"
	log.Infoln(selectQueryStr)
	var tmpId int
	rErr := db.QueryRow(selectQueryStr).Scan(&tmpId)
	return tmpId, rErr
}

func updateDTOAddressOnTableAdresse(c *gin.Context) {
	addr_uuid := c.Param("addr_uuid")
	var addressOfPerson DTOAddress

	err := c.BindJSON(&addressOfPerson)
	if err != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}

	validateDTOAddress, _ := validateDTOAddress(addressOfPerson)

	if strings.Compare(addr_uuid, addressOfPerson.UUID.String()) != 0 {
		AbortWithStatusJSON(c, 400, "uuid from URL and uuid as JSON in body does not fit: "+addr_uuid+" vs "+addressOfPerson.UUID.String())
		return
	}

	id, err2 := getIDFromUUIDOrCreateDummyData("adresse", addressOfPerson.UUID)

	if err2 != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}

	idOfCountry, err3 := getCountryIdByNameAKABezeichnung(addressOfPerson.Country)

	if err3 != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}

	updateSQLStr := `
	UPDATE adresse SET 
		land = ` + strconv.Itoa(idOfCountry) + `, 
		plz = '` + EscapeTick(addressOfPerson.ZIP) + `', 
		ort = '` + EscapeTick(addressOfPerson.City) + `', 
		strasse = '` + EscapeTick(addressOfPerson.Street) + `', 
		tel1 = '` + EscapeTick(addressOfPerson.Phone_Home) + `', 
		tel2 = '` + EscapeTick(addressOfPerson.Phone_Work) + `', 
		tel3 = '` + EscapeTick(addressOfPerson.Phone_Mobile) + `', 
		email1 = '` + EscapeTick(addressOfPerson.Email) + `', 
		email2 = '` + EscapeTick(addressOfPerson.Email2) + `' 
	WHERE id = ` + strconv.Itoa(id) + `
	`
	log.Infoln(updateSQLStr)

	_, err4 := db.Exec(updateSQLStr)
	if err4 != nil {
		AbortWithStatusJSON(c, 400, err4.Error())
		return
	} else {
		if validateDTOAddress {
			CJSON(c, 200, addressOfPerson)
		} else {
			CJSON(c, 202, addressOfPerson)
		}
		return
	}
}

func insertDTOAddressIntoTableAdresse(c *gin.Context) {
	addr_uuid := c.Param("addr_uuid")
	var addressOfPerson DTOAddress

	err := c.BindJSON(&addressOfPerson)
	if err != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}

	validateDTOAddress, _ := validateDTOAddress(addressOfPerson)

	if strings.Compare(addr_uuid, addressOfPerson.UUID.String()) != 0 {
		AbortWithStatusJSON(c, 400, "uuid from URL and uuid as JSON in body does not fit: "+addr_uuid+" vs "+addressOfPerson.UUID.String())
		return
	}

	idOfCountry, err3 := getCountryIdByNameAKABezeichnung(addressOfPerson.Country)

	if err3 != nil {
		AbortWithStatusJSON(c, 400, err3.Error())
		return
	}

	updateSQLStr := `
	INSERT INTO adresse (
		uuid,
		land,
		plz,
		ort,
		strasse,
		tel1,
		tel2,
		tel3,
		email1,
		email2
		)
	VALUES ('` + addressOfPerson.UUID.String() +
		`', ` + strconv.Itoa(idOfCountry) +
		`, '` + EscapeTick(addressOfPerson.ZIP) +
		`', '` + EscapeTick(addressOfPerson.City) +
		`', '` + EscapeTick(addressOfPerson.Street) +
		`', '` + EscapeTick(addressOfPerson.Phone_Home) +
		`', '` + EscapeTick(addressOfPerson.Phone_Work) +
		`', '` + EscapeTick(addressOfPerson.Phone_Mobile) +
		`', '` + EscapeTick(addressOfPerson.Email) +
		`', '` + EscapeTick(addressOfPerson.Email2) +
		`')`

	log.Infoln(updateSQLStr)

	_, err4 := db.Exec(updateSQLStr)
	if err4 != nil {
		AbortWithStatusJSON(c, 400, err4.Error())
		return
	} else {
		if validateDTOAddress {
			CJSON(c, 200, addressOfPerson)
		} else {
			CJSON(c, 202, addressOfPerson)
		}
		return
	}
}

func deleteDTOAddress(c *gin.Context) {
	addr_uuid := c.Param("addr_uuid")
	if isValidUUIDofTable(addr_uuid, "adressen") {
		deleteSQLStr := "delete from adressen where uuid = ?"
		deleteDTOGeneric(c, addr_uuid, deleteSQLStr)
	} else if isValidUUIDofTable(addr_uuid, "adresse") {
		deleteSQLStr := "delete from adresse where uuid = ?"
		deleteDTOGeneric(c, addr_uuid, deleteSQLStr)
	} else {
		AbortWithStatusJSON(c, 404, "addr_uuid: "+addr_uuid+" was neither found in table adresse nor table adressen")
		return
	}
}

// table person and table mitgliedschaft
func getDTOPlayerLicense(c *gin.Context) {
	license_uuid := c.Param("license_uuid")
	// var clubmember DTOClubMember
	var playerlicense DTOPlayerLicense

	if isValidUUID(license_uuid) {
		myUuid, _ := uuid.Parse(license_uuid)
		playerlicense.UUID = myUuid

		var count string
		var sqlSelectQueryCount string = `select count(*) from mitgliedschaft where uuid = '` + myUuid.String() + `'`
		errDBExec := db.QueryRow(sqlSelectQueryCount).Scan(&count)
		log.Info(sqlSelectQueryCount)

		if errDBExec != nil {
			AbortWithStatusJSON(c, 500, errDBExec.Error())
			return
		} else {
			if strings.Compare(count, "0") == 0 {
				AbortWithStatusJSON(c, 404, "player license (table mitgliedschaft) with the uuid was not found in the database: "+myUuid.String())
				return
			}
		}

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
				mitgliedschaft.uuid = '` + license_uuid + `'`
		log.Infoln(sqlSelectQuery)

		var memberFrom string
		var memberUntil string
		var licenseState string

		err := db.QueryRow(sqlSelectQuery).
			Scan(
				&playerlicense.Club_UUID,
				&playerlicense.Person_UUID,
				&memberFrom,
				&memberUntil,
				&licenseState,
				&playerlicense.Member_Nr,
			)

		log.Infoln(memberFrom)
		log.Infoln(memberUntil)
		var parseError error
		if strings.Compare(memberFrom, "") != 0 {
			playerlicense.LicenseValidFrom, parseError = parseStringToCivilTime(memberFrom)
			if parseError != nil {
				AbortWithStatusJSON(c, 500, parseError.Error())
				return
			}
		}
		if strings.Compare(memberUntil, "") != 0 {
			playerlicense.LicenseValidUntil, parseError = parseStringToCivilTime(memberUntil)
			if parseError != nil {
				AbortWithStatusJSON(c, 500, parseError.Error())
				return
			}
		}

		myLicenseState, err := strconv.Atoi(licenseState)
		if err != nil {
			AbortWithStatusJSON(c, 500, err.Error())
			return
		}
		playerlicense.License_State = LicenseStateToString(LicenseState(myLicenseState))

		if err != nil {
			AbortWithStatusJSON(c, 500, err.Error())
			return
		}

		CJSON(c, 200, playerlicense)

	} else {
		AbortWithStatusJSON(c, 400, "uuid is not valid: "+license_uuid)
		return
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
	var sqlQuerySelectID = "SELECT id FROM " + tableName + " WHERE uuid = '" + myUuid.String() + "'"
	var tmpId int
	log.Infoln(sqlQuerySelectID)
	rErr = db.QueryRow(sqlQuerySelectID).Scan(&tmpId)
	return tmpId, rErr
}

func getIDFromUUIDOrCreateDummyData(tableName string, myUuid uuid.UUID) (id int, rErr error) {
	id, rErr = getIDFromUUID(tableName, myUuid)

	if rErr == nil {
		return id, rErr
	} else {
		var sqlInsertDummyData = "INSERT INTO " + tableName + " (uuid) VALUES ('" + myUuid.String() + "')"
		log.Infoln(sqlInsertDummyData)
		_, rErr = db.Exec(sqlInsertDummyData)

		if rErr == nil {
			return getIDFromUUID(tableName, myUuid)
		} else {
			return -1, rErr
		}
	}
}

func putDTOPlayerLicense(c *gin.Context) {
	license_uuid := c.Param("license_uuid")
	var playerlicense DTOPlayerLicense

	err := c.BindJSON(&playerlicense)
	if err != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}

	if strings.Compare(license_uuid, playerlicense.UUID.String()) != 0 {
		AbortWithStatusJSON(c, 400, "uuid from URL and uuid as JSON in body does not fit: "+license_uuid+" vs "+playerlicense.UUID.String())
		return
	}

	if isValidUUID(playerlicense.UUID.String()) {
		myUuid, parseErr := uuid.Parse(playerlicense.UUID.String())

		if parseErr != nil {
			AbortWithStatusJSON(c, 400, parseErr.Error())
			return
		}

		if !isValidUUID(myUuid.String()) {
			AbortWithStatusJSON(c, 400, errors.New("myUuid is not a valid UUID: "+myUuid.String()))
			return
		}

		var count string
		var sqlSelectQuery string = `select count(*) from mitgliedschaft where uuid = '` + myUuid.String() + `'`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		log.Info(sqlSelectQuery)

		if errDBExec != nil {
			AbortWithStatusJSON(c, 500, err.Error())
			return
		} else {

			var person_id, _ = getIDFromUUIDOrCreateDummyData("person", playerlicense.Person_UUID)
			var organisation_id, _ = getIDFromUUIDOrCreateDummyData("organisation", playerlicense.Club_UUID)
			var fromStr = CivilTimeToString(playerlicense.LicenseValidFrom)
			var untilStr = CivilTimeToString(playerlicense.LicenseValidUntil)

			if strings.Compare(count, "0") == 0 { // insert

				var sqlInsertQuery string
				if fromStr == "" && untilStr == "" {
					sqlInsertQuery = `
						INSERT INTO mitgliedschaft (
							uuid,
							person,
							organisation,
							stat1,
							spielernummer)
						VALUES ('` + playerlicense.UUID.String() +
						`', ` + strconv.Itoa(person_id) +
						`, '` + strconv.Itoa(organisation_id) +
						`', '` + strconv.Itoa(int(getLicenseStateFromString(playerlicense.License_State))) +
						`', ` + strconv.Itoa(playerlicense.Member_Nr) + `)
					`
				} else if fromStr == "" {
					sqlInsertQuery = `
						INSERT INTO mitgliedschaft (
							uuid,
							person,
							organisation,
							bis,
							stat1,
							spielernummer)
						VALUES ('` + playerlicense.UUID.String() +
						`', ` + strconv.Itoa(person_id) +
						`, '` + strconv.Itoa(organisation_id) +
						`', '` + untilStr +
						`', '` + strconv.Itoa(int(getLicenseStateFromString(playerlicense.License_State))) +
						`', ` + strconv.Itoa(playerlicense.Member_Nr) + `)
					`
				} else if untilStr == "" {
					sqlInsertQuery = `
						INSERT INTO mitgliedschaft (
							uuid,
							person,
							organisation,
							von,
							stat1,
							spielernummer)
						VALUES ('` + playerlicense.UUID.String() +
						`', ` + strconv.Itoa(person_id) +
						`, '` + strconv.Itoa(organisation_id) +
						`', '` + fromStr +
						`', '` + strconv.Itoa(int(getLicenseStateFromString(playerlicense.License_State))) +
						`', ` + strconv.Itoa(playerlicense.Member_Nr) + `)
					`
				} else {
					sqlInsertQuery = `
						INSERT INTO mitgliedschaft (
							uuid,
							person,
							organisation,
							von,
							bis,
							stat1,
							spielernummer)
						VALUES ('` + playerlicense.UUID.String() +
						`', ` + strconv.Itoa(person_id) +
						`, '` + strconv.Itoa(organisation_id) +
						`', '` + fromStr +
						`', '` + untilStr +
						`', '` + strconv.Itoa(int(getLicenseStateFromString(playerlicense.License_State))) +
						`', ` + strconv.Itoa(playerlicense.Member_Nr) + `)
					`
				}
				log.Infoln(sqlInsertQuery)

				_, err3 := db.Exec(sqlInsertQuery)

				if err3 != nil {
					AbortWithStatusJSON(c, 400, err3.Error())
					return
				} else {
					CJSON(c, 200, playerlicense)
					return
				}
			} else if strings.Compare(count, "1") == 0 { // update

				var sqlUpdateQuery string
				if fromStr == "" && untilStr == "" {
					sqlUpdateQuery = `
						UPDATE mitgliedschaft set 
							person = ` + strconv.Itoa(person_id) + `,
							organisation = ` + strconv.Itoa(organisation_id) + `,
							von = null,
							bis = null,
							stat1 = ` + strconv.Itoa(int(getLicenseStateFromString(playerlicense.License_State))) + `,
							spielernummer = ` + strconv.Itoa(playerlicense.Member_Nr) + `
						WHERE uuid = '` + playerlicense.UUID.String() + `'
					`
				} else if fromStr == "" {
					sqlUpdateQuery = `
						UPDATE mitgliedschaft set 
							person = ` + strconv.Itoa(person_id) + `,
							organisation = ` + strconv.Itoa(organisation_id) + `,
							von = null,
							bis = '` + untilStr + `',
							stat1 = ` + strconv.Itoa(int(getLicenseStateFromString(playerlicense.License_State))) + `,
							spielernummer = ` + strconv.Itoa(playerlicense.Member_Nr) + `
						WHERE uuid = '` + playerlicense.UUID.String() + `'
					`
				} else if untilStr == "" {
					sqlUpdateQuery = `
						UPDATE mitgliedschaft set 
							person = ` + strconv.Itoa(person_id) + `,
							organisation = ` + strconv.Itoa(organisation_id) + `,
							von = '` + fromStr + `',
							bis = null,
							stat1 = ` + strconv.Itoa(int(getLicenseStateFromString(playerlicense.License_State))) + `,
							spielernummer = ` + strconv.Itoa(playerlicense.Member_Nr) + `
						WHERE uuid = '` + playerlicense.UUID.String() + `'
					`
				} else {
					sqlUpdateQuery = `
						UPDATE mitgliedschaft set 
							person = ` + strconv.Itoa(person_id) + `,
							organisation = ` + strconv.Itoa(organisation_id) + `,
							von = '` + fromStr + `',
							bis = '` + untilStr + `',
							stat1 = ` + strconv.Itoa(int(getLicenseStateFromString(playerlicense.License_State))) + `,
							spielernummer = ` + strconv.Itoa(playerlicense.Member_Nr) + `
						WHERE uuid = '` + playerlicense.UUID.String() + `'
					`
				}
				log.Infoln(sqlUpdateQuery)

				_, err4 := db.Exec(sqlUpdateQuery)
				if err4 != nil {
					AbortWithStatusJSON(c, 400, err4.Error())
					return
				} else {
					CJSON(c, 200, playerlicense)
					return
				}
			} else {
				AbortWithStatusJSON(c, 500, errors.New("panic, more than 1 club member with same uuid: "+myUuid.String()))
				return
			}
		}
	} else {
		AbortWithStatusJSON(c, 400, errors.New("uuid is not valid "+playerlicense.UUID.String()))
		return
	}
}

func deleteDTOPlayerLicense(c *gin.Context) {
	license_uuid := c.Param("license_uuid")
	deleteSQLStr := "delete from mitgliedschaft where uuid = ?"
	deleteDTOGeneric(c, license_uuid, deleteSQLStr)
}

// table funktion
func getDTOClubOfficial(c *gin.Context) {
	official_uuid := c.Param("official_uuid")
	var clubofficial DTOClubOfficial

	if isValidUUID(official_uuid) {
		myUuid, _ := uuid.Parse(official_uuid)
		clubofficial.UUID = myUuid

		var count string
		var sqlSelectQueryCount string = `select count(*) from funktion where uuid = '` + myUuid.String() + `'`
		errDBExec := db.QueryRow(sqlSelectQueryCount).Scan(&count)
		log.Info(sqlSelectQueryCount)

		if errDBExec != nil {
			AbortWithStatusJSON(c, 500, errDBExec.Error())
			return
		} else {
			if strings.Compare(count, "0") == 0 {
				AbortWithStatusJSON(c, 404, "club official with the uuid was not found in the database: "+myUuid.String())
				return
			}
		}

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
				funktion.uuid = '` + official_uuid + `'`
		log.Infoln(sqlSelectQuery)

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

		if err != nil {
			AbortWithStatusJSON(c, 500, err.Error())
			return
		}

		var parseErrClubUUID error
		clubofficial.Club_UUID, parseErrClubUUID = uuid.Parse(tmpClubUuid)
		if parseErrClubUUID != nil {
			AbortWithStatusJSON(c, 500, parseErrClubUUID.Error())
			return
		}
		var parseErrPersonUUID error
		clubofficial.Person_UUID, parseErrPersonUUID = uuid.Parse(tmpPersonUuid)
		if parseErrPersonUUID != nil {
			AbortWithStatusJSON(c, 500, parseErrPersonUUID.Error())
			return
		}

		log.Infoln(validFrom)
		log.Infoln(validUntil)

		var parseError error
		if strings.Compare(validFrom, "") != 0 {
			clubofficial.Valid_From, parseError = parseStringToCivilTime(validFrom)
			if parseError != nil {
				AbortWithStatusJSON(c, 500, parseError.Error())
				return
			}
		}
		if strings.Compare(validUntil, "") != 0 {
			clubofficial.Valid_Until, parseError = parseStringToCivilTime(validUntil)
			if parseError != nil {
				AbortWithStatusJSON(c, 500, parseError.Error())
				return
			}
		}

		CJSON(c, 200, clubofficial)
		return

	} else {
		AbortWithStatusJSON(c, 400, errors.New("uuid is not valid "+official_uuid))
		return
	}
}

func putDTOClubOfficial(c *gin.Context) {
	official_uuid := c.Param("official_uuid")
	var clubofficial DTOClubOfficial

	err := c.BindJSON(&clubofficial)
	if err != nil {
		AbortWithStatusJSON(c, 400, err.Error())
		return
	}

	if strings.Compare(official_uuid, clubofficial.UUID.String()) != 0 {
		AbortWithStatusJSON(c, 400, "uuid from URL and uuid as JSON in body does not fit: "+official_uuid+" vs "+clubofficial.UUID.String())
		return
	}

	if isValidUUID(clubofficial.UUID.String()) {
		myUuid, parseErr := uuid.Parse(clubofficial.UUID.String())

		if parseErr != nil {
			AbortWithStatusJSON(c, 400, parseErr.Error())
			return
		}

		var count string
		var sqlSelectQuery string = `select count(*) from funktion where uuid = '` + myUuid.String() + `'`
		errDBExec := db.QueryRow(sqlSelectQuery).Scan(&count)
		log.Infoln(sqlSelectQuery)

		if errDBExec != nil {
			AbortWithStatusJSON(c, 500, err.Error())
			return
		} else {

			var person_id, errPersonId = getIDFromUUIDOrCreateDummyData("person", clubofficial.Person_UUID)
			if errPersonId != nil {
				AbortWithStatusJSON(c, 400, errPersonId.Error())
				return
			}
			var organisation_id, errOrganisationId = getIDFromUUIDOrCreateDummyData("organisation", clubofficial.Club_UUID)
			if errOrganisationId != nil {
				AbortWithStatusJSON(c, 400, errOrganisationId.Error())
				return
			}
			var fromStr = CivilTimeToString(clubofficial.Valid_From)
			var untilStr = CivilTimeToString(clubofficial.Valid_Until)
			var funktion = ClubRoleNameToFunktion(clubofficial.Role_Name)

			if strings.Compare(count, "0") == 0 { // insert

				// TODO add funktion as id as well -> erledigt Michael
				var sqlInsertQuery string
				if fromStr == "" && untilStr == "" {
					sqlInsertQuery = `
						INSERT INTO funktion (
							uuid,
							organisation,
							person,
							funktion,
							funktionsalias)
						VALUES ('` + clubofficial.UUID.String() +
						`', ` + strconv.Itoa(organisation_id) +
						`, ` + strconv.Itoa(person_id) +
						`, ` + funktion +
						`, '` + clubofficial.Role_Name +
						`')
					`
				} else if fromStr == "" {
					sqlInsertQuery = `
						INSERT INTO funktion (
							uuid,
							organisation,
							person,
							funktion,
							funktionsalias,
							bis)
						VALUES ('` + clubofficial.UUID.String() +
						`', ` + strconv.Itoa(organisation_id) +
						`,` + strconv.Itoa(person_id) +
						`, ` + funktion +
						`, '` + clubofficial.Role_Name +
						`', '` + untilStr + `')
					`
				} else if untilStr == "" {
					sqlInsertQuery = `
						INSERT INTO funktion (
							uuid,
							organisation,
							person,
							funktion,
							funktionsalias,
							von)
						VALUES ('` + clubofficial.UUID.String() +
						`', ` + strconv.Itoa(organisation_id) +
						`,` + strconv.Itoa(person_id) +
						`, ` + funktion +
						`, '` + clubofficial.Role_Name +
						`', '` + fromStr +
						`')
					`
				} else {
					sqlInsertQuery = `
						INSERT INTO funktion (
							uuid,
							organisation,
							person,
							funktion,
							funktionsalias,
							von,
							bis)
						VALUES ('` + clubofficial.UUID.String() +
						`', ` + strconv.Itoa(organisation_id) +
						`,` + strconv.Itoa(person_id) +
						`, ` + funktion +
						`, '` + clubofficial.Role_Name +
						`', '` + fromStr +
						`', '` + untilStr + `')
					`
				}
				log.Infoln(sqlInsertQuery)

				_, err3 := db.Exec(sqlInsertQuery)

				if err3 != nil {
					AbortWithStatusJSON(c, 400, err3.Error())
					return
				} else {
					CJSON(c, 200, clubofficial)
					return
				}
			} else if strings.Compare(count, "1") == 0 { // update

				var sqlUpdateQuery string
				if fromStr == "" && untilStr == "" {
					sqlUpdateQuery = `
						UPDATE funktion set 
							organisation = ` + strconv.Itoa(organisation_id) + `,
							person = ` + strconv.Itoa(person_id) + `,
							funktion = ` + funktion + `,
							funktionsalias = '` + EscapeTick(clubofficial.Role_Name) + `',
							von = null,
							bis = null
						WHERE uuid = '` + clubofficial.UUID.String() + `'
					`
				} else if fromStr == "" {
					sqlUpdateQuery = `
						UPDATE funktion set 
							organisation = ` + strconv.Itoa(organisation_id) + `,
							person = ` + strconv.Itoa(person_id) + `,
							funktion = ` + funktion + `,
							funktionsalias = '` + EscapeTick(clubofficial.Role_Name) + `',
							von = null,
							bis = '` + untilStr + `'
						WHERE uuid = '` + clubofficial.UUID.String() + `'
					`
				} else if untilStr == "" {
					sqlUpdateQuery = `
						UPDATE funktion set 
							organisation = ` + strconv.Itoa(organisation_id) + `,
							person = ` + strconv.Itoa(person_id) + `,
							funktion = ` + funktion + `,
							funktionsalias = '` + EscapeTick(clubofficial.Role_Name) + `',
							von = '` + fromStr + `',
							bis = null
						WHERE uuid = '` + clubofficial.UUID.String() + `'
					`
				} else {
					sqlUpdateQuery = `
						UPDATE funktion set 
							organisation = ` + strconv.Itoa(organisation_id) + `,
							person = ` + strconv.Itoa(person_id) + `,
							funktion = ` + funktion + `,
							funktionsalias = '` + EscapeTick(clubofficial.Role_Name) + `',
							von = '` + fromStr + `',
							bis = '` + untilStr + `'
						WHERE uuid = '` + clubofficial.UUID.String() + `'
					`
				}
				log.Infoln(sqlUpdateQuery)

				_, err4 := db.Exec(sqlUpdateQuery)
				if err4 != nil {
					AbortWithStatusJSON(c, 400, err4.Error())
					return
				} else {
					CJSON(c, 200, clubofficial)
					return
				}
			} else {
				AbortWithStatusJSON(c, 500, errors.New("panic, more than 1 club offical with same uuid: "+myUuid.String()))
				return
			}
		}
	} else {
		AbortWithStatusJSON(c, 400, errors.New("uuid is not valid"+clubofficial.UUID.String()))
		return
	}
}

func deleteDTOClubOfficial(c *gin.Context) {
	official_uuid := c.Param("official_uuid")
	deleteSQLStr := "delete from funktion where uuid = ?"
	deleteDTOGeneric(c, official_uuid, deleteSQLStr)
}

func putDTORegion(c *gin.Context) {
	CJSON(c, 204, "Mivis does not support Region, so ignore and no handling of input.")
}

func getDTOClubMember(c *gin.Context) {
	CJSON(c, 204, "Mivis does not support Club Member, so ignore and no handling of input.")
}

func putDTOClubMember(c *gin.Context) {
	CJSON(c, 204, "Mivis does not support Club Member, so ignore and no handling of input.")
}

func deleteDTOClubMember(c *gin.Context) {
	CJSON(c, 204, "Mivis does not support Club Member, so ignore and no handling of input.")
}

func main() {

	envFile, _ := godotenv.Read(".env")

	yourMySQLdatabasepassword = envFile["yourMySQLdatabasepassword"]
	basicAuthUsername = envFile["basicAuthUsername"]
	basicAuthPassword = envFile["basicAuthPassword"]

	var dataSourceName = "portal:" + yourMySQLdatabasepassword + "@tcp(127.0.0.1:3306)/mvdsb"
	var err error
	db, err = sql.Open("mysql", dataSourceName)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	var SQLQuerySet string = `SET NAMES utf8mb4`
	_, err = db.Exec(SQLQuerySet)
	if err != nil {
		panic(err.Error())
	}
	
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(LoggingMiddleware())

	// authorized := router.Group("/api")
	authorized := router.Group("/api", verifyTokenController())

	//authorized := router.Group("/api", gin.BasicAuth(gin.Accounts{
	// 	basicAuthUsername: basicAuthPassword,
	// }))

	// public := router.Group("/public")
	// public.POST("/register", registerLoginUser)
	// public.POST("/login", loginUser)
	// authorized := router.Group("/api")
	// authorized.Use(jwtAuthMiddleware())

	authorized.PUT("/regions/:reg_uuid", putDTORegion)

	authorized.GET("/federations/:fed_uuid", getDTOFederation)
	authorized.PUT("/federations/:fed_uuid", putDTOFederation)
	authorized.DELETE("/federations/:fed_uuid", deleteDTOFederation)

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

	// router.Run(":3030")
	router.RunTLS(":3030", envFile["SSL_CERTFILE_FULLCHAIN"], envFile["SSL_PRIVATE_KEYFILE"])
}
