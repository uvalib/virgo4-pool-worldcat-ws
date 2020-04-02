package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/uvalib/virgo4-parser/v4parser"
)

type providerDetails struct {
	Provider    string `json:"provider"`
	Label       string `json:"label,omitempty"`
	HomepageURL string `json:"homepage_url,omitempty"`
	LogoURL     string `json:"logo_url,omitempty"`
}

type poolProviders struct {
	Providers []providerDetails `json:"providers"`
}

// ProvidersHandler returns a list of access_url providers for JMRL
func (svc *ServiceContext) providersHandler(c *gin.Context) {
	p := poolProviders{Providers: make([]providerDetails, 0)}
	p.Providers = append(p.Providers, providerDetails{
		Provider:    "worldcat",
		Label:       "WorldCat",
		LogoURL:     "/assets/wclogo.png",
		HomepageURL: "https://uva.worldcat.org",
	})
	c.JSON(http.StatusOK, p)
}

// Search accepts a search POST, transforms the query into JMRL format and perfoms the search
func (svc *ServiceContext) search(c *gin.Context) {
	log.Printf("Search requested")
	var req SearchRequest
	if err := c.BindJSON(&req); err != nil {
		log.Printf("ERROR: unable to parse search request: %s", err.Error())
		c.String(http.StatusBadRequest, "invalid request")
		return
	}

	acceptLang := strings.Split(c.GetHeader("Accept-Language"), ",")[0]
	if acceptLang == "" {
		acceptLang = "en-US"
	}

	log.Printf("Raw query: %s, %+v %+v", req.Query, req.Pagination, req.Sort)
	valid, errors := v4parser.Validate(req.Query)
	if valid == false {
		log.Printf("ERROR: Query [%s] is not valid: %s", req.Query, errors)
		c.String(http.StatusBadRequest, "Malformed search")
		return
	}

	paginationStr := fmt.Sprintf("startRecord=%d&maximumRecords=%d", req.Pagination.Start, req.Pagination.Rows)
	sortKey := fmt.Sprintf("sortKeys=%s", getSortKey(req.Sort))

	//  Convert V4 query into WorldCat format
	// EX: keyword: {(calico OR "tortoise shell") AND cats}
	// DATES: date: {1987} OR date: {AFTER 2010} OR date: {BEFORE 1990} OR date: {1987 TO 1990}
	// NOTE: the v4 language allows multiple instances of each criteria. WorldCat does not, so the raw query must be
	// broken up by criteria and then combined so each only appears once
	parsedQ, dErr := convertDateCriteria(req.Query)
	if dErr != nil {
		log.Printf("ERROR: invalid date in query %s", req.Query)
		c.String(http.StatusBadRequest, dErr.Error())
		return
	}
	parsedQ = strings.ReplaceAll(parsedQ, "{", "")
	parsedQ = strings.ReplaceAll(parsedQ, "}", "")
	parsedQ = strings.ReplaceAll(parsedQ, "keyword:", "srw.kw =")
	parsedQ = strings.ReplaceAll(parsedQ, "title:", "srw.ki =")
	parsedQ = strings.ReplaceAll(parsedQ, "author:", "srw.au =")
	parsedQ = strings.ReplaceAll(parsedQ, "subject:", "srw.su =")
	parsedQ = strings.ReplaceAll(parsedQ, "identifier:", "srw.bn =")
	parsedQ = strings.TrimSpace(parsedQ)
	log.Printf("Parsed query: %s", parsedQ)
	if parsedQ == "" || parsedQ == "*" {
		c.String(http.StatusNotImplemented, "Blank or * searches are not supported")
		return
	}

	qURL := fmt.Sprintf("%s/search/worldcat/sru?recordSchema=dc&query=%s&%s&%s&wskey=%s",
		svc.WCAPI, url.QueryEscape(parsedQ), paginationStr, sortKey, svc.WCKey)
	rawResp, respErr := svc.apiGet(qURL)
	if respErr != nil {
		c.String(respErr.StatusCode, respErr.Message)
		return
	}

	// log.Printf("RESP [%s]", rawResp)

	c.String(http.StatusOK, string(rawResp))
}

// Facets placeholder implementaion for a V4 facet POST.
func (svc *ServiceContext) facets(c *gin.Context) {
	log.Printf("Facets requested, but WorldCat does not support this")
	c.JSON(http.StatusNotImplemented, "Facets are not supported")
}

// GetResource will get a WorkdCat resource by ID
func (svc *ServiceContext) getResource(c *gin.Context) {
	id := c.Param("id")
	log.Printf("Resource %s details requested", id)
	c.String(http.StatusNotImplemented, "under construction")
}

func convertDateCriteria(query string) (string, error) {
	for true {
		dateIdx := strings.Index(query, "date:")
		if dateIdx == -1 {
			break
		}
		chunk := query[dateIdx:]
		i0 := strings.Index(chunk, "{")
		i1 := strings.Index(chunk, "}")
		pre := strings.Trim(query[0:dateIdx], " ")
		post := strings.Trim(query[dateIdx+i1+1:], " ")

		// EX: date: {1987} OR date: {AFTER 2010} OR date: {BEFORE 1990} OR date: {1987 TO 1990}
		qt := strings.Trim(chunk[i0+1:i1], " ")
		if strings.Contains(qt, "AFTER") {
			year := strings.Trim(strings.ReplaceAll(qt, "AFTER", ""), " ")
			if len(year) != 4 {
				return "", errors.New("Only 4 digit year is accepted in a date search")
			}
			qt = "srw.yr > " + year
		} else if strings.Contains(qt, "BEFORE") {
			year := strings.Trim(strings.ReplaceAll(qt, "BEFORE", ""), " ")
			if len(year) != 4 {
				return "", errors.New("Only 4 digit year is accepted in a date search")
			}
			qt = "srw.yr < " + year
		} else if strings.Contains(qt, "TO") {
			years := strings.Split(qt, " TO ")
			if len(years[0]) != 4 || len(years[1]) != 4 {
				return "", errors.New("Only 4 digit year is accepted in a date search")
			}
			qt = fmt.Sprintf("srw.yr >= %s and srw.yr <= %s", years[0], years[1])
		} else {
			year := strings.Trim(qt, " ")
			if len(year) != 4 {
				return "", errors.New("Only 4 digit year is accepted in a date search")
			}
			qt = "srw.yr = " + year
		}

		query = fmt.Sprintf("%s %s %s", pre, qt, post)
	}
	return query, nil
}

func getSortKey(sort SortOrder) string {
	if sort.SortID == SortAuthor.String() {
		if sort.Order == "asc" {
			return "Author"
		}
		return "Author,,0"
	}
	if sort.SortID == SortTitle.String() {
		if sort.Order == "asc" {
			return "Title"
		}
		return "Title,,0"
	}
	if sort.SortID == SortDate.String() {
		if sort.Order == "asc" {
			return "Date"
		}
		return "Date,,0"
	}
	return "relevance"
}
