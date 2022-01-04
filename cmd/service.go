package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/uvalib/virgo4-api/v4api"
	"github.com/uvalib/virgo4-jwt/v4jwt"
	"golang.org/x/text/language"
)

// OCLC contains data necessary to get and use OCLC auth tokens
type OCLC struct {
	Key         string
	Secret      string
	AuthURL     string
	MetadataAPI string
	Token       string
	Expires     time.Time
}

// ServiceContext contains common data used by all handlers
type ServiceContext struct {
	Version    string
	Port       int
	WCKey      string
	WCAPI      string
	JWTKey     string
	I18NBundle *i18n.Bundle
	HTTPClient *http.Client
	OCLC       OCLC
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

	svc.OCLC.AuthURL = cfg.OCLCAuthURL
	svc.OCLC.Key = cfg.OCLCKey
	svc.OCLC.Secret = cfg.OCLCSecret
	svc.OCLC.MetadataAPI = cfg.OCLCMetadataAPI

	log.Printf("Init localization")
	svc.I18NBundle = i18n.NewBundle(language.English)
	svc.I18NBundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	svc.I18NBundle.MustLoadMessageFile("./i18n/active.en.toml")
	svc.I18NBundle.MustLoadMessageFile("./i18n/active.es.toml")

	log.Printf("Create HTTP Client")
	defaultTransport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   2 * time.Second,
			KeepAlive: 600 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 2 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
	}
	svc.HTTPClient = &http.Client{
		Transport: defaultTransport,
		Timeout:   5 * time.Second,
	}

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

	pingReq, _ := http.NewRequest("GET", svc.WCAPI, nil)
	resp, postErr := svc.HTTPClient.Do(pingReq)
	if resp != nil {
		defer resp.Body.Close()
	}
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

	resp := v4api.PoolIdentity{Attributes: make([]v4api.PoolAttribute, 0)}
	resp.Name = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "PoolName"})
	resp.Description = localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "PoolDescription"})
	resp.Mode = "record"

	resp.Attributes = append(resp.Attributes, v4api.PoolAttribute{Name: "logo_url", Supported: true, Value: "/assets/wclogo.png"})
	resp.Attributes = append(resp.Attributes, v4api.PoolAttribute{Name: "external_url", Supported: true, Value: "https://www.worldcat.org/"})
	resp.Attributes = append(resp.Attributes, v4api.PoolAttribute{Name: "facets", Supported: false})
	resp.Attributes = append(resp.Attributes, v4api.PoolAttribute{Name: "sorting", Supported: true})
	resp.Attributes = append(resp.Attributes, v4api.PoolAttribute{Name: "availability", Supported: true})
	resp.Attributes = append(resp.Attributes, v4api.PoolAttribute{Name: "item_message", Supported: true, Value: `This resource is not held by the UVA Library. Development is ongoing to utilize WorldCat to enable easy Interlibrary Loan requests.`})

	resp.SortOptions = make([]v4api.SortOption, 0)
	resp.SortOptions = append(resp.SortOptions, v4api.SortOption{ID: v4api.SortRelevance.String(), Label: "Relevance"})
	resp.SortOptions = append(resp.SortOptions, v4api.SortOption{ID: v4api.SortDate.String(), Label: "Date Published", Asc: "oldest first", Desc: "newest first"})
	resp.SortOptions = append(resp.SortOptions, v4api.SortOption{ID: v4api.SortTitle.String(), Label: "Title", Asc: "A-Z", Desc: "Z-A"})
	resp.SortOptions = append(resp.SortOptions, v4api.SortOption{ID: v4api.SortAuthor.String(), Label: "Author", Asc: "A-Z", Desc: "Z-A"})

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
func (svc *ServiceContext) apiGet(tgtURL string, bearerToken string) ([]byte, *RequestError) {
	log.Printf("WorldCat API GET request: %s", tgtURL)
	startTime := time.Now()
	getReq, _ := http.NewRequest("GET", tgtURL, nil)
	if bearerToken != "" {
		log.Printf("INFO: adding bearer token to api request")
		getReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bearerToken))
	}
	rawResp, rawErr := svc.HTTPClient.Do(getReq)
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

func (svc *ServiceContext) oclcTokenRequest() *RequestError {
	log.Printf("INFO: request OCLC token from %s", svc.OCLC.AuthURL)
	svc.OCLC.Expires = time.Now()
	svc.OCLC.Token = ""
	startTime := time.Now()
	req, _ := http.NewRequest("POST", svc.OCLC.AuthURL, nil)
	req.SetBasicAuth(svc.OCLC.Key, svc.OCLC.Secret)
	rawResp, rawErr := svc.HTTPClient.Do(req)
	resp, err := handleAPIResponse(svc.OCLC.AuthURL, rawResp, rawErr)
	elapsedNanoSec := time.Since(startTime)
	elapsedMS := int64(elapsedNanoSec / time.Millisecond)

	if err != nil {
		log.Printf("ERROR: failed response from OCLC auth reques %s %d. Elapsed Time: %d (ms). %s",
			svc.OCLC.AuthURL, err.StatusCode, elapsedMS, err.Message)
		return err
	}

	log.Printf("INFO: successful response from GET %s. Elapsed Time: %d (ms)", svc.OCLC.AuthURL, elapsedMS)
	log.Printf("INFO: update OCLC auth token data")
	var authResponse struct {
		Token   string `json:"access_token"`
		Expires string `json:"expires_at"`
	}
	parseErr := json.Unmarshal(resp, &authResponse)
	if parseErr != nil {
		log.Printf("ERROR: unable to parse auth response: %s", parseErr.Error())
	}

	expTime, _ := time.Parse("2006-02-02 15:04:05Z", authResponse.Expires)
	log.Printf("INFO: oclc token expires %+v", expTime)
	svc.OCLC.Token = authResponse.Token
	svc.OCLC.Expires = expTime

	return nil
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
