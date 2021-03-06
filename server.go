package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

type (
	user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
)

// it will store the  data coming from user input
var (
	users = map[int]*user{}
	seq   = 1
)

type bodyDumpResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

//----------
// Handlers
//----------

func createUser(c echo.Context) error {
	u := &user{
		ID: seq,
	}

	if err := c.Bind(u); err != nil {
		return err
	}

	users[u.ID] = u
	seq++
	return c.JSON(http.StatusCreated, u)
}

func getUser(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	return c.JSON(http.StatusOK, users[id])
}

func getAllUsers(c echo.Context) error {
	println(users)
	return c.JSON(http.StatusOK, users)
}

func updateUser(c echo.Context) error {
	u := new(user)
	if err := c.Bind(u); err != nil {
		return err
	}
	id, _ := strconv.Atoi(c.Param("id"))
	users[id].Name = u.Name
	return c.JSON(http.StatusOK, users[id])
}

func deleteUser(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	delete(users, id)
	return c.NoContent(http.StatusNoContent)
}

func checkForXSSPayload(dat map[string]interface{}) bool {
	isRequestMalacious := false

	for key, value := range dat {
		log.Printf("key: " + key + " ")
		// print array properties
		arr, ok := value.([]interface{})
		if ok {
			log.Printf("value: array [")
			for _, arrVal := range arr {
				// recurse subobjects in the array
				subobj, ok := arrVal.(map[string]interface{})
				if ok {
					checkForXSSPayload(subobj)
				} else {
					// print other values
					log.Printf("value: %+v\n", arrVal)
				}
			}
			log.Printf("]")
		}

		// recurse subobjects
		subobj, ok := value.(map[string]interface{})
		if ok {
			checkForXSSPayload(subobj)
		} else {
			var re = regexp.MustCompile(`<("[^"]*"|'[^']*'|[^'">])*>`)
			Value := fmt.Sprintf("%v", value)
			Value, err := url.QueryUnescape(Value)
			if err != nil {
				log.Fatal(err)
			}

			if re.MatchString(Value) {
				log.Printf("XSS attack, malicious script")
				isRequestMalacious = true
				break
			} else {
				log.Printf("payload is secure")
			}
			log.Printf("value: %+v\n", value)
		}
	}
	return isRequestMalacious
}

func CheckForXSSAttack() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			log.Printf("c inside CheckForXSSAttack %s\n", c)
			if c.Request().Method == "PUT" || c.Request().Method == "POST" {

				var err error

				var f map[string]interface{}

				reqBody := []byte{}
				if c.Request().Body != nil { // Read
					reqBody, _ = ioutil.ReadAll(c.Request().Body)
				}
				c.Request().Body = ioutil.NopCloser(bytes.NewBuffer(reqBody)) // Reset

				err = json.Unmarshal([]byte(reqBody), &f)
				if err != nil {
					log.Printf("error while unmarshing JSON", err)
				}

				if checkForXSSPayload(f) == true {
					return echo.NewHTTPError(http.StatusBadRequest, "Bad Request")
				}
			}
			return next(c)
		}
	}
}

//----------
// Main function
//----------
func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "deny",
		ContentSecurityPolicy: "default-src 'self'",
	}))

	e.Use(CheckForXSSAttack())

	// Routes
	e.POST("/users", createUser)
	e.GET("/users/:id", getUser)
	e.GET("/users/all", getAllUsers)
	e.PUT("/users/:id", updateUser)
	e.DELETE("/users/:id", deleteUser)

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
