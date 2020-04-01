package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/uvalib/virgo4-jwt/v4jwt"
	"golang.org/x/text/language"
)

// ServiceContext contains common data used by all handlers
type ServiceContext struct {
	Version    string
	Port       int
	WCKey      string
	WCAPI      string
	JWTKey     string
	I18NBundle *i18n.Bundle
}

// RequestError contains http status code and message for and API request
type RequestError struct {
	StatusCode int
	Message    string
}

// InitializeService will initialize the service context based on the config parameters.
// Any pools found in the DB will be added to the context and polled for status.
// Any errors are FATAL.
func InitializeService(version string, cfg *ServiceConfig) *ServiceContext {
	log.Printf("Initializing Service")
	svc := ServiceContext{Version: version, WCKey: cfg.WCKey, WCAPI: cfg.WCAPI, JWTKey: cfg.JWTKey}

	log.Printf("Init localization")
	svc.I18NBundle = i18n.NewBundle(language.English)
	svc.I18NBundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	svc.I18NBundle.MustLoadMessageFile("./i18n/active.en.toml")
	svc.I18NBundle.MustLoadMessageFile("./i18n/active.es.toml")

	return &svc
}

// IgnoreFavicon is a dummy to handle browser favicon requests without warnings
func (svc *ServiceContext) ignoreFavicon(c *gin.Context) {
	// no-op; just here to prevent errors when request made from browser
}

// GetVersion reports the version of the serivce
func (svc *ServiceContext) getVersion(c *gin.Context) {
	build := "unknown"
	// working directory is the bin directory, and build tag is in the root
	files, _ := filepath.Glob("../buildtag.*")
	if len(files) == 1 {
		build = strings.Replace(files[0], "../buildtag.", "", 1)
	}

	vMap := make(map[string]string)
	vMap["version"] = svc.Version
	vMap["build"] = build
	c.JSON(http.StatusOK, vMap)
}

// HealthCheck reports the health of the serivce
func (svc *ServiceContext) healthCheck(c *gin.Context) {
	type hcResp struct {
		Healthy bool   `json:"healthy"`
		Message string `json:"message,omitempty"`
	}
	hcMap := make(map[string]hcResp)

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	pingReq, _ := http.NewRequest("GET", svc.WCAPI, nil)
	resp, postErr := client.Do(pingReq)
	if postErr != nil {
		hcMap["worldcat_api"] = hcResp{Healthy: false, Message: postErr.Error()}
	} else if resp.StatusCode != 200 {
		hcMap["worldcat_api"] = hcResp{Healthy: false, Message: resp.Status}
	} else {
		hcMap["worldcat_api"] = hcResp{Healthy: true}
	}

	c.JSON(http.StatusOK, hcMap)
}

// IdentifyHandler returns localized identity information for this pool
func (svc *ServiceContext) identifyHandler(c *gin.Context) {
	acceptLang := strings.Split(c.GetHeader("Accept-Language"), ",")[0]
	if acceptLang == "" {
		acceptLang = "en-US"
	}
	log.Printf("Identify request Accept-Language %s", acceptLang)
	localizer := i18n.NewLocalizer(svc.I18NBundle, acceptLang)

	resp := PoolIdentity{Attributes: make([]PoolAttribute, 0)}
	resp.Name = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "PoolName"})
	resp.Description = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "PoolDescription"})
	resp.Mode = "record"
	resp.Attributes = append(resp.Attributes, PoolAttribute{Name: "logo_url", Supported: true, Value: "/assets/wclogo.png"})
	resp.Attributes = append(resp.Attributes, PoolAttribute{Name: "external_url", Supported: true, Value: "https://uva.worldcat.org/"})
	resp.Attributes = append(resp.Attributes, PoolAttribute{Name: "uva_ils", Supported: false})
	resp.Attributes = append(resp.Attributes, PoolAttribute{Name: "facets", Supported: false})
	resp.Attributes = append(resp.Attributes, PoolAttribute{Name: "cover_images", Supported: false})
	resp.Attributes = append(resp.Attributes, PoolAttribute{Name: "course_reserves", Supported: false})
	resp.Attributes = append(resp.Attributes, PoolAttribute{Name: "sorting", Supported: true})
	resp.SortOptions = make([]SortOption, 0)
	resp.SortOptions = append(resp.SortOptions, SortOption{ID: "SortRelevance", Label: "Relevance"})
	resp.SortOptions = append(resp.SortOptions, SortOption{ID: "SortDatePublished", Label: "Date Published"})
	resp.SortOptions = append(resp.SortOptions, SortOption{ID: "SortTitle", Label: "Title"})
	resp.SortOptions = append(resp.SortOptions, SortOption{ID: "SortAuthor", Label: "Author"})

	c.JSON(http.StatusOK, resp)
}

// getBearerToken is a helper to extract the user auth token from the Auth header
func getBearerToken(authorization string) (string, error) {
	components := strings.Split(strings.Join(strings.Fields(authorization), " "), " ")

	// must have two components, the first of which is "Bearer", and the second a non-empty token
	if len(components) != 2 || components[0] != "Bearer" || components[1] == "" {
		return "", fmt.Errorf("Invalid Authorization header: [%s]", authorization)
	}

	return components[1], nil
}

// AuthMiddleware is a middleware handler that verifies presence of a
// user Bearer token in the Authorization header.
func (svc *ServiceContext) authMiddleware(c *gin.Context) {
	tokenStr, err := getBearerToken(c.Request.Header.Get("Authorization"))
	if err != nil {
		log.Printf("Authentication failed: [%s]", err.Error())
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if tokenStr == "undefined" {
		log.Printf("Authentication failed; bearer token is undefined")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	log.Printf("Validating JWT auth token...")
	v4Claims, jwtErr := v4jwt.Validate(tokenStr, svc.JWTKey)
	if jwtErr != nil {
		log.Printf("JWT signature for %s is invalid: %s", tokenStr, jwtErr.Error())
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// add the parsed claims and signed JWT string to the request context so other handlers can access it.
	c.Set("jwt", tokenStr)
	c.Set("claims", v4Claims)
	log.Printf("got bearer token: [%s]: %+v", tokenStr, v4Claims)
}

// APIGet sends a GET to the WorldCat API and returns results a byte array
func (svc *ServiceContext) apiGet(tgtURL string) ([]byte, *RequestError) {
	log.Printf("JMRL API GET request: %s", tgtURL)
	startTime := time.Now()
	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	getReq, _ := http.NewRequest("GET", tgtURL, nil)
	rawResp, rawErr := client.Do(getReq)
	resp, err := handleAPIResponse(tgtURL, rawResp, rawErr)
	elapsedNanoSec := time.Since(startTime)
	elapsedMS := int64(elapsedNanoSec / time.Millisecond)

	if err != nil {
		log.Printf("ERROR: Failed response from GET %s %d. Elapsed Time: %d (ms). %s",
			tgtURL, err.StatusCode, elapsedMS, err.Message)
	} else {
		log.Printf("Successful response from GET %s. Elapsed Time: %d (ms)", tgtURL, elapsedMS)
	}
	return resp, err
}

func handleAPIResponse(URL string, resp *http.Response, err error) ([]byte, *RequestError) {
	if err != nil {
		status := http.StatusBadRequest
		errMsg := err.Error()
		if strings.Contains(err.Error(), "Timeout") {
			status = http.StatusRequestTimeout
			errMsg = fmt.Sprintf("%s timed out", URL)
		} else if strings.Contains(err.Error(), "connection refused") {
			status = http.StatusServiceUnavailable
			errMsg = fmt.Sprintf("%s refused connection", URL)
		}
		return nil, &RequestError{StatusCode: status, Message: errMsg}
	} else if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		status := resp.StatusCode
		errMsg := string(bodyBytes)
		return nil, &RequestError{StatusCode: status, Message: errMsg}
	}

	defer resp.Body.Close()
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	return bodyBytes, nil
}
