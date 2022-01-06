package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uvalib/virgo4-api/v4api"
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

type wcSearchResponse struct {
	XMLName xml.Name   `xml:"searchRetrieveResponse"`
	Count   int        `xml:"numberOfRecords"`
	Records []wcRecord `xml:"records>record>recordData>oclcdcs"`
}

type wcRecord struct {
	XMLName     xml.Name `xml:"oclcdcs"`
	ID          string   `xml:"recordIdentifier"`
	Date        string   `xml:"date"`
	Language    string   `xml:"language"`
	ISBN        []string `xml:"identifier"`
	Creator     []string `xml:"creator,omitempty"`
	Contributor []string `xml:"contributor,omitempty"`
	Description []string `xml:"description,omitempty"`
	Subjects    []string `xml:"subject,omitempty"`
	Title       []string `xml:"title,omitempty"`
	Type        []string `xml:"type,omitempty"`
	Formats     []string `xml:"format,omitempty"`
	Publishers  []string `xml:"publisher,omitempty"`
}

// ProvidersHandler returns a list of access_url providers for JMRL
func (svc *ServiceContext) providersHandler(c *gin.Context) {
	p := poolProviders{Providers: make([]providerDetails, 0)}
	p.Providers = append(p.Providers, providerDetails{
		Provider:    "worldcat",
		Label:       "WorldCat",
		LogoURL:     "/assets/wclogo.png",
		HomepageURL: "https://www.worldcat.org/",
	})
	p.Providers = append(p.Providers, providerDetails{
		Provider:    "hathitrust",
		Label:       "Hathi Trust Digital Library",
		LogoURL:     "/assets/hathitrust.png",
		HomepageURL: "https://www.hathitrust.org/",
	})
	p.Providers = append(p.Providers, providerDetails{
		Provider:    "proquest",
		Label:       "ProQuest U.S. Congressional Hearings Digital Collection",
		LogoURL:     "/assets/proquest.jpg",
		HomepageURL: "https://www.proquest.com/",
	})
	p.Providers = append(p.Providers, providerDetails{
		Provider:    "google",
		Label:       "Google Books",
		LogoURL:     "/assets/google.png",
		HomepageURL: "https://books.google.com/",
	})
	p.Providers = append(p.Providers, providerDetails{
		Provider:    "vlebooks",
		Label:       "VLeBooks",
		LogoURL:     "/assets/vlebooks.png",
		HomepageURL: "https://www.vlebooks.com/",
	})
	p.Providers = append(p.Providers, providerDetails{
		Provider:    "canadiana",
		Label:       "Canadiana",
		LogoURL:     "/assets/canadiana.png",
		HomepageURL: "http://www.canadiana.ca/",
	})
	p.Providers = append(p.Providers, providerDetails{
		Provider:    "overdrive",
		Label:       "Overdrive",
		LogoURL:     "/assets/overdrive.png",
		HomepageURL: "https://www.overdrive.com",
	})
	c.JSON(http.StatusOK, p)
}

// Search accepts a search POST, transforms the query into JMRL format and perfoms the search
func (svc *ServiceContext) search(c *gin.Context) {
	log.Printf("Search requested")
	var req v4api.SearchRequest
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

	// journal queries are not supported
	if strings.Contains(req.Query, "journal_title:") {
		log.Printf("ERROR: journal title queries are not supported")
		c.String(http.StatusNotImplemented, "Journal Title queries are not supported")
		return
	}

	paginationStr := fmt.Sprintf("startRecord=%d&maximumRecords=%d", req.Pagination.Start, req.Pagination.Rows)
	sortKey := fmt.Sprintf("sortKeys=%s", getSortKey(req.Sort))

	// Convert V4 query into WorldCat format
	// EX: keyword: {(calico OR "tortoise shell") AND cats}
	// DATES: date: {1987} OR date: {AFTER 2010} OR date: {BEFORE 1990} OR date: {1987 TO 1990}
	parsedQ, dErr := convertDateCriteria(req.Query)
	if dErr != nil {
		log.Printf("ERROR: invalid date in query %s: %s", req.Query, dErr.Error())
		c.String(http.StatusBadRequest, dErr.Error())
		return
	}
	parsedQ = strings.ReplaceAll(parsedQ, "{", "")
	parsedQ = strings.ReplaceAll(parsedQ, "}", "")
	parsedQ = strings.ReplaceAll(parsedQ, "keyword:", "srw.kw all")
	parsedQ = strings.ReplaceAll(parsedQ, "title:", "srw.ti all")
	parsedQ = strings.ReplaceAll(parsedQ, "author:", "srw.au all")
	parsedQ = strings.ReplaceAll(parsedQ, "subject:", "srw.su all")
	parsedQ = strings.ReplaceAll(parsedQ, "identifier:", "srw.bn =")
	parsedQ = strings.TrimSpace(parsedQ)
	log.Printf("Raw parsed query [%s]", parsedQ)
	if parsedQ == "srw.kw all" || parsedQ == "srw.kw all *" {
		c.String(http.StatusNotImplemented, "At least 3 characters are required.")
		return
	}

	// WorldCat does not support filtering. If a filter is specified in the search, return 0 hits
	if len(req.Filters) > 0 || strings.Contains(req.Query, "filter:") {
		log.Printf("Filters specified in search, return no matches")
		v4Resp := &v4api.PoolResult{ElapsedMS: 0, Confidence: "low"}
		v4Resp.Groups = make([]v4api.Group, 0)
		v4Resp.Pagination = v4api.Pagination{Start: 0, Total: 0, Rows: 0}
		v4Resp.StatusCode = http.StatusOK
		v4Resp.ContentLanguage = acceptLang
		c.JSON(http.StatusOK, v4Resp)
		return
	}

	// if a basic search that is ISBN is done (just a number) do an identifier search too
	if strings.Contains(parsedQ, "srw.") &&
		strings.Index(parsedQ, "srw.") == strings.LastIndex(parsedQ, "srw.") &&
		strings.Index(parsedQ, "srw.") == strings.Index(parsedQ, "srw.kw") {
		param := strings.Trim(strings.Split(parsedQ, "all")[1], " ")
		if _, err := strconv.Atoi(param); err == nil {
			log.Printf("%s looks like a keyword query for an identifier; add identifier search", parsedQ)
			parsedQ += fmt.Sprintf(" OR srw.bn = %s", param)
		}
	}

	// skip any UVA libraries
	log.Printf("Final parsed query: %s", parsedQ)
	parsedQ += " NOT srw.li = VA@  NOT srw.li = VAL NOT srw.li = VAM"

	startTime := time.Now()
	qURL := fmt.Sprintf("%s/search/worldcat/sru?recordSchema=dc&query=%s&%s&%s&wskey=%s",
		svc.WCAPI, url.QueryEscape(parsedQ), paginationStr, sortKey, svc.WCKey)
	rawResp, respErr := svc.apiGet(qURL, "")
	if respErr != nil {
		c.String(respErr.StatusCode, respErr.Message)
		return
	}

	// successful search; setup response
	elapsedNanoSec := time.Since(startTime)
	elapsedMS := int64(elapsedNanoSec / time.Millisecond)
	v4Resp := &v4api.PoolResult{ElapsedMS: elapsedMS, Confidence: "low"}
	v4Resp.Groups = make([]v4api.Group, 0)
	if req.Sort.SortID == "" {
		v4Resp.Sort.SortID = v4api.SortRelevance.String()
		v4Resp.Sort.Order = "desc"
	} else {
		v4Resp.Sort = req.Sort
	}

	wcResp := &wcSearchResponse{}
	fmtErr := xml.Unmarshal(rawResp, wcResp)
	if fmtErr != nil {
		log.Printf("ERROR: Invalid response from WorldCat API: %s", fmtErr.Error())
		log.Printf("Response: %s", rawResp)
		v4Resp.StatusCode = http.StatusInternalServerError
		v4Resp.StatusMessage = fmtErr.Error()
		c.JSON(v4Resp.StatusCode, v4Resp)
		return
	}

	v4Resp.Pagination = v4api.Pagination{Start: req.Pagination.Start, Total: wcResp.Count,
		Rows: len(wcResp.Records)}
	for _, wcRec := range wcResp.Records {
		groupRec := v4api.Group{Value: wcRec.ID, Count: 1}
		groupRec.Records = make([]v4api.Record, 0)
		record := v4api.Record{}
		record.Fields = getResultFields(&wcRec)
		groupRec.Records = append(groupRec.Records, record)
		v4Resp.Groups = append(v4Resp.Groups, groupRec)
	}

	if wcResp.Count > 0 {
		v4Resp.Confidence = "medium"
	}

	v4Resp.StatusCode = http.StatusOK
	v4Resp.ContentLanguage = acceptLang
	c.JSON(http.StatusOK, v4Resp)
}

// Facets placeholder implementaion for a V4 facet POST.
func (svc *ServiceContext) facets(c *gin.Context) {
	log.Printf("Facets requested, but WorldCat does not support this")
	empty := make(map[string]interface{})
	empty["facets"] = make([]v4api.Facet, 0)
	c.JSON(http.StatusOK, empty)
}

// GetResource will get a WorkdCat resource by ID
func (svc *ServiceContext) getResource(c *gin.Context) {
	id := c.Param("id")
	log.Printf("Resource %s details requested", id)
	qURL := fmt.Sprintf("%s/content/%s?recordSchema=dc&serviceLevel=full&wskey=%s",
		svc.WCAPI, id, svc.WCKey)
	rawResp, respErr := svc.apiGet(qURL, "")
	if respErr != nil {
		c.String(respErr.StatusCode, respErr.Message)
		return
	}

	wcResp := &wcRecord{}
	fmtErr := xml.Unmarshal(rawResp, wcResp)
	if fmtErr != nil {
		log.Printf("ERROR: Invalid response from WorldCat API: %s", fmtErr.Error())
		log.Printf("Response: %s", rawResp)
		c.String(http.StatusInternalServerError, fmtErr.Error())
		return
	}

	var jsonResp struct {
		Fields []v4api.RecordField `json:"fields"`
	}
	jsonResp.Fields = getResultFields(wcResp)

	log.Printf("INFO: lookup generalFormat for %s", id)
	err := svc.refreshOCLCAuth()
	if err != nil {
		log.Printf("INFO: unable to refresh OCLC auth: %s", err.Error())
		c.JSON(http.StatusOK, jsonResp)
		return
	}
	genFmt, err := svc.getGeneralFormat(id)
	if err != nil {
		log.Printf("ERROR: unable to get general format for %s: %s", id, err.Error())
	} else {
		var fmtJSON struct {
			GeneralFormat  string `json:"generalFormat"`
			SpecificFormat string `json:"specificFormat"`
		}
		parseErr := json.Unmarshal(genFmt, &fmtJSON)
		if parseErr != nil {
			log.Printf("ERROR: unable to parse general format response for %s: %s", id, parseErr.Error())
		} else {
			log.Printf("INFO: item %s has  format %s:%s", id, fmtJSON.GeneralFormat, fmtJSON.SpecificFormat)
			gf := v4api.RecordField{Name: "general_format", Type: "format", Label: "General Format",
				Value: fmtJSON.GeneralFormat, Display: "optional"}
			jsonResp.Fields = append(jsonResp.Fields, gf)
			sf := v4api.RecordField{Name: "specific_format", Type: "format", Label: "Specific Format",
				Value: fmtJSON.GeneralFormat, Display: "optional"}
			jsonResp.Fields = append(jsonResp.Fields, sf)
		}
	}

	c.JSON(http.StatusOK, jsonResp)
}

func (svc *ServiceContext) getGeneralFormat(id string) ([]byte, error) {
	resp, respErr := svc.apiGet(fmt.Sprintf("%s/%s", svc.OCLC.MetadataAPI, id), svc.OCLC.Token)
	if respErr != nil {
		svc.OCLC.Token = ""
		svc.OCLC.Expires = time.Now()
		return nil, errors.New(respErr.Message)
	}
	return resp, nil
}

func (svc *ServiceContext) refreshOCLCAuth() error {
	log.Printf("INFO: check OCLC auth token")
	now := time.Now()
	log.Printf("INFO: token expire %s vs time now %s", svc.OCLC.Expires.Format(time.UnixDate), now.Format(time.UnixDate))
	if svc.OCLC.Token == "" || svc.OCLC.Expires.After(now) {
		log.Printf("INFO: requesting new OCLC auth token")
		err := svc.oclcTokenRequest()
		if err != nil {
			return errors.New(err.Message)
		}
		log.Printf("INFO: oclc auth successfully updated")
	} else {
		log.Printf("INFO: oclc auth is valid and unexpired")
	}
	return nil
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
			yearStr := strings.Trim(strings.ReplaceAll(qt, "AFTER", ""), " ")
			year, err := extractYear(yearStr)
			if err != nil {
				return "", err
			}
			qt = "srw.yr > " + year
		} else if strings.Contains(qt, "BEFORE") {
			yearStr := strings.Trim(strings.ReplaceAll(qt, "BEFORE", ""), " ")
			year, err := extractYear(yearStr)
			if err != nil {
				return "", err
			}
			qt = "srw.yr < " + year
		} else if strings.Contains(qt, "TO") {
			years := strings.Split(qt, " TO ")
			yearFrom, err := extractYear(years[0])
			if err != nil {
				return "", errors.New("Starting year is invalid")
			}
			yearTo, err := extractYear(years[0])
			if err != nil {
				return "", errors.New("Ending year is invalid")
			}
			qt = fmt.Sprintf("srw.yr >= %s and srw.yr <= %s", yearFrom, yearTo)
		} else {
			yearStr := strings.Trim(qt, " ")
			year, err := extractYear(yearStr)
			if err != nil {
				return "", err
			}
			qt = "srw.yr = " + year
		}

		query = fmt.Sprintf("%s %s %s", pre, qt, post)
	}
	return query, nil
}

func extractYear(yearStr string) (string, error) {
	parts := strings.Split(yearStr, "-")
	year := parts[0]
	match, _ := regexp.Match(`\d{4}`, []byte(year))
	if !match {
		return "", errors.New("Only 4 digit year is accepted in a date search")
	}
	return year, nil
}

func getSortKey(sort v4api.SortOrder) string {
	if sort.SortID == v4api.SortAuthor.String() {
		if sort.Order == "asc" {
			return "Author"
		}
		return "Author,,0"
	}
	if sort.SortID == v4api.SortTitle.String() {
		if sort.Order == "asc" {
			return "Title"
		}
		return "Title,,0"
	}
	if sort.SortID == v4api.SortDate.String() {
		if sort.Order == "asc" {
			return "Date"
		}
		return "Date,,0"
	}
	return "relevance"
}

func getResultFields(wcRec *wcRecord) []v4api.RecordField {
	fields := make([]v4api.RecordField, 0)
	f := v4api.RecordField{Name: "id", Type: "identifier", Label: "Identifier",
		Value: wcRec.ID, Display: "optional", CitationPart: "id"}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "publication_date", Type: "publication_date", Label: "Publication Date",
		Value: wcRec.Date, CitationPart: "published_date"}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "language", Type: "language", Label: "Language",
		Value: wcRec.Language, Visibility: "detailed", CitationPart: "language"}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "title", Type: "title", Label: "Title", Value: wcRec.Title[0], CitationPart: "title"}
	fields = append(fields, f)

	online := false
	for _, val := range wcRec.ISBN {
		if strings.Contains(val, "http") == false {
			f = v4api.RecordField{Name: "isbn", Type: "isbn", Label: "ISBN", Value: val, CitationPart: "serial_number"}
			fields = append(fields, f)
		} else {
			if strings.Contains(val, "api.overdrive") || strings.Contains(val, "[institution]") {
				log.Printf("WARN: Skipping URL that appears invalid: %s", val)
			} else {
				online = true
				onlineF := v4api.RecordField{Name: "access_url", Type: "url", Label: "Online Access", Value: val, Provider: "worldcat"}
				if strings.Contains(val, "hathitrust") {
					log.Printf("Online access with HathiTrust")
					onlineF.Provider = "hathitrust"
				} else if strings.Contains(val, "proquest") {
					log.Printf("Online access with ProQuest")
					onlineF.Provider = "proquest"
				} else if strings.Contains(val, "google") {
					log.Printf("Online access with Google")
					onlineF.Provider = "google"
				} else if strings.Contains(val, "vlebooks") {
					log.Printf("Online access with VLeBooks")
					onlineF.Provider = "vlebooks"
				} else if strings.Contains(val, "canadiana") {
					log.Printf("Online access with Canadiana")
					onlineF.Provider = "canadiana"
				} else if strings.Contains(val, "overdrive") {
					log.Printf("Online access with Overdrive")
					onlineF.Provider = "overdrive"
				} else {
					log.Printf("Online access: %s", val)
				}

				fields = append(fields, onlineF)
			}
		}
	}

	if online {
		/*
			availF := v4api.RecordField{Name: "availability", Type: "availability", Label: "Availability", Value: "Online"}
			fields = append(fields, availF)
		*/
	} else {
		/*
			availF := v4api.RecordField{Name: "availability", Type: "availability", Label: "Availability", Value: "By Request"}
			fields = append(fields, availF)
		*/
	}

	f = v4api.RecordField{Name: "worldcat_url", Type: "url", Label: "More Details", Provider: "worldcat",
		Value: fmt.Sprintf("http://worldcat.org/oclc/%s", wcRec.ID), Visibility: "detailed"}
	fields = append(fields, f)

	for _, val := range wcRec.Creator {
		f = v4api.RecordField{Name: "author", Type: "author", Label: "Author", Value: html.UnescapeString(val), CitationPart: "author"}
		fields = append(fields, f)
	}
	for _, val := range wcRec.Contributor {
		f = v4api.RecordField{Name: "author", Type: "author", Label: "Author", Value: html.UnescapeString(val), CitationPart: "author"}
		fields = append(fields, f)
	}

	for _, val := range wcRec.Subjects {
		f = v4api.RecordField{Name: "subject", Type: "subject", Label: "Subject", Value: val, Visibility: "detailed", CitationPart: "subject"}
		fields = append(fields, f)
	}

	f = v4api.RecordField{Name: "description", Type: "summary", Label: "Description",
		Value: strings.Join(wcRec.Description, " "), CitationPart: "abstract"}
	fields = append(fields, f)

	for _, val := range wcRec.Publishers {
		f = v4api.RecordField{Name: "publisher", Label: "Publisher", Visibility: "detailed", Value: val, CitationPart: "publisher"}
	}

	for _, val := range wcRec.Formats {
		f = v4api.RecordField{Name: "format", Label: "Format", Visibility: "detailed", Value: val}
	}

	for _, val := range wcRec.Type {
		f = v4api.RecordField{Name: "type", Label: "Type", Visibility: "detailed", Value: val}
	}

	return fields
}
