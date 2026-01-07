package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uvalib/virgo4-api/v4api"
	"github.com/uvalib/virgo4-parser/v4parser"
)

type wcBriefRecord struct {
	OclcNumber          string   `json:"oclcNumber"`
	Title               string   `json:"title"`
	Creator             string   `json:"creator,omitempty"`
	Date                string   `json:"date"`
	MachineReadableDate string   `json:"machineReadableDate"`
	Language            string   `json:"language"`
	GeneralFormat       string   `json:"generalFormat"`
	SpecificFormat      string   `json:"specificFormat"`
	Edition             string   `json:"edition,omitempty"`
	Publisher           string   `json:"publisher,omitempty"`
	PublicationPlace    string   `json:"publicationPlace,omitempty"`
	Isbns               []string `json:"isbns,omitempty"`
}

type wcDetailRecord struct {
	Identifier struct {
		OclcNumber string   `json:"oclcNumber"`
		Isbns      []string `json:"isbns"`
	} `json:"identifier"`
	Title struct {
		MainTitles []struct {
			Text string `json:"text"`
		} `json:"mainTitles"`
		SeriesTitles []struct {
			SeriesTitle string `json:"seriesTitle"`
		} `json:"seriesTitles"`
	} `json:"title"`
	Contributor struct {
		StatementOfResponsibility struct {
			Text string `json:"text"`
		} `json:"statementOfResponsibility"`
	} `json:"contributor"`
	Subjects []struct {
		SubjectName struct {
			Text string `json:"text"`
		} `json:"subjectName"`
	} `json:"subjects"`
	Publishers []struct {
		PublisherName struct {
			Text string `json:"text"`
		} `json:"publisherName"`
		PublicationPlace string `json:"publicationPlace"`
	} `json:"publishers"`
	Date struct {
		PublicationDate string `json:"publicationDate"`
	} `json:"date"`
	Language struct {
		ItemLanguage string `json:"itemLanguage"`
	} `json:"language"`
	Note struct {
		GeneralNotes []struct {
			Text  string `json:"text"`
			Local string `json:"local"`
		} `json:"generalNotes"`
	} `json:"note"`
	Format struct {
		GeneralFormat  string   `json:"generalFormat"`
		SpecificFormat string   `json:"specificFormat"`
		MaterialTypes  []string `json:"materialTypes"`
	} `json:"format"`
	Description struct {
		PhysicalDescription string `json:"physicalDescription"`
		Summaries           []struct {
			Text string `json:"text"`
		} `json:"summaries"`
		PeerReviewed string `json:"peerReviewed"`
	} `json:"description"`
}

type wcSearchResponse struct {
	NumberOfRecords int             `json:"numberOfRecords"`
	BriefRecords    []wcBriefRecord `json:"briefRecords"`
}

// ProvidersHandler returns a list of access_url providers for WorldCat
func (svc *ServiceContext) providersHandler(c *gin.Context) {
	p := v4api.PoolProviders{Providers: make([]v4api.Provider, 0)}
	p.Providers = append(p.Providers, v4api.Provider{
		Provider:    "worldcat",
		Label:       "WorldCat",
		LogoURL:     "/assets/wclogo.png",
		HomepageURL: "https://www.worldcat.org/",
	})
	p.Providers = append(p.Providers, v4api.Provider{
		Provider:    "hathitrust",
		Label:       "Hathi Trust Digital Library",
		LogoURL:     "/assets/hathitrust.png",
		HomepageURL: "https://www.hathitrust.org/",
	})
	p.Providers = append(p.Providers, v4api.Provider{
		Provider:    "proquest",
		Label:       "ProQuest U.S. Congressional Hearings Digital Collection",
		LogoURL:     "/assets/proquest.jpg",
		HomepageURL: "https://www.proquest.com/",
	})
	p.Providers = append(p.Providers, v4api.Provider{
		Provider:    "google",
		Label:       "Google Books",
		LogoURL:     "/assets/google.png",
		HomepageURL: "https://books.google.com/",
	})
	p.Providers = append(p.Providers, v4api.Provider{
		Provider:    "vlebooks",
		Label:       "VLeBooks",
		LogoURL:     "/assets/vlebooks.png",
		HomepageURL: "https://www.vlebooks.com/",
	})
	p.Providers = append(p.Providers, v4api.Provider{
		Provider:    "canadiana",
		Label:       "Canadiana",
		LogoURL:     "/assets/canadiana.png",
		HomepageURL: "http://www.canadiana.ca/",
	})
	p.Providers = append(p.Providers, v4api.Provider{
		Provider:    "overdrive",
		Label:       "Overdrive",
		LogoURL:     "/assets/overdrive.png",
		HomepageURL: "https://www.overdrive.com",
	})
	c.JSON(http.StatusOK, p)
}

// Search accepts a search POST, transforms the query into WorldCat format and perfoms the search
func (svc *ServiceContext) search(c *gin.Context) {
	log.Printf("Search requested")
	var req v4api.SearchRequest
	if err := c.BindJSON(&req); err != nil {
		log.Printf("ERROR: unable to parse search request: %s", err.Error())
		c.String(http.StatusBadRequest, "invalid request")
		return
	}

	log.Printf("Raw query: %s, %+v %+v", req.Query, req.Pagination, req.Sort)
	valid, errors := v4parser.Validate(req.Query)
	if valid == false {
		log.Printf("ERROR: Query [%s] is not valid: %s", req.Query, errors)
		c.String(http.StatusBadRequest, "Malformed search")
		return
	}

	// journal_title, fulltext, and series queries are not supported
	// We mark these messages as WARNING's because they are expected
	if strings.Contains(req.Query, "journal_title:") {
		log.Printf("WARNING: journal title queries are not supported")
		c.String(http.StatusNotImplemented, "Journal Title queries are not supported")
		return
	}
	if strings.Contains(req.Query, "fulltext:") {
		log.Printf("WARNING: full text queries are not supported")
		c.String(http.StatusNotImplemented, "Full Text queries are not supported")
		return
	}
	if strings.Contains(req.Query, "series:") {
		log.Printf("WARNING: series queries are not supported")
		c.String(http.StatusNotImplemented, "Series queries are not supported")
		return
	}

	paginationStr := fmt.Sprintf("offset=%d&limit=%d", req.Pagination.Start+1, req.Pagination.Rows)
	sortKey := fmt.Sprintf("orderBy=%s", getSortKey(req.Sort))

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
	parsedQ = strings.ReplaceAll(parsedQ, "keyword: ", "kw:")
	parsedQ = strings.ReplaceAll(parsedQ, "title: ", "ti:")
	parsedQ = strings.ReplaceAll(parsedQ, "author: ", "au:")
	parsedQ = strings.ReplaceAll(parsedQ, "subject: ", "su:")
	parsedQ = strings.ReplaceAll(parsedQ, "identifier: ", "no:")
	parsedQ = strings.TrimSpace(parsedQ)
	log.Printf("Raw parsed query [%s]", parsedQ)
	if parsedQ == "kw:" || parsedQ == "kw:*" {
		c.String(http.StatusNotImplemented, "At least 3 characters are required.")
		return
	}

	// worldcat does not supprt date only queries
	if strings.Contains(parsedQ, "yr:") && strings.Contains(parsedQ, "kw:") == false && strings.Contains(parsedQ, "ti:") == false &&
		strings.Contains(parsedQ, "au:") == false && strings.Contains(parsedQ, "au:") == false && strings.Contains(parsedQ, "no:") == false {
		log.Printf("INFO; unspoorted date-only query detected [%s]; return no results", parsedQ)
		v4Resp := &v4api.PoolResult{ElapsedMS: 0, Confidence: "low"}
		v4Resp.Groups = make([]v4api.Group, 0)
		v4Resp.Pagination = v4api.Pagination{Start: 0, Total: 0, Rows: 0}
		v4Resp.StatusCode = http.StatusOK
		c.JSON(http.StatusOK, v4Resp)
	}

	// WorldCat does not support filtering. If a filter is specified in the search, return 0 hits
	// Note: when doing a next page request, the request contains:
	//       Filters:[{PoolID:worldcat Facets:[]}]
	//       accept this configuration
	filtersSpecified := false
	if len(req.Filters) > 1 {
		filtersSpecified = true
	} else if len(req.Filters) == 1 {
		filtersSpecified = len(req.Filters[0].Facets) > 0
	}
	if filtersSpecified || strings.Contains(req.Query, "filter:") {
		log.Printf("INFO: filters specified in search, return no matches")
		v4Resp := &v4api.PoolResult{ElapsedMS: 0, Confidence: "low"}
		v4Resp.Groups = make([]v4api.Group, 0)
		v4Resp.Pagination = v4api.Pagination{Start: 0, Total: 0, Rows: 0}
		v4Resp.StatusCode = http.StatusOK
		c.JSON(http.StatusOK, v4Resp)
		return
	}

	// skip any UVA libraries
	parsedQ += " NOT li:VA@  NOT li:VAL NOT li:VAM"
	log.Printf("Final parsed query: %s", parsedQ)

	startTime := time.Now()
	qURL := fmt.Sprintf("%s/worldcat/search/v2/brief-bibs?q=%s&%s&%s", svc.OCLC.API, url.QueryEscape(parsedQ), paginationStr, sortKey)
	rawResp, respErr := svc.apiGet(qURL, svc.OCLC.Token)
	if respErr != nil {
		c.String(respErr.StatusCode, respErr.Message)
		return
	}

	log.Printf("%s", rawResp)

	// successful search; setup response
	elapsedNanoSec := time.Since(startTime)
	elapsedMS := int64(elapsedNanoSec / time.Millisecond)
	v4Resp := &v4api.PoolResult{ElapsedMS: elapsedMS, Confidence: "medium"}
	v4Resp.Groups = make([]v4api.Group, 0)
	if req.Sort.SortID == "" {
		v4Resp.Sort.SortID = v4api.SortRelevance.String()
		v4Resp.Sort.Order = "desc"
	} else {
		v4Resp.Sort = req.Sort
	}

	wcResp := &wcSearchResponse{}
	if err := json.Unmarshal(rawResp, &wcResp); err != nil {
		log.Printf("ERROR: Invalid response from WorldCat API: %s", err.Error())
		log.Printf("Response: %s", rawResp)
		v4Resp.StatusCode = http.StatusInternalServerError
		v4Resp.StatusMessage = err.Error()
		c.JSON(v4Resp.StatusCode, v4Resp)
		return
	}

	v4Resp.Pagination = v4api.Pagination{
		Start: req.Pagination.Start,
		Total: wcResp.NumberOfRecords,
		Rows:  len(wcResp.BriefRecords),
	}
	for _, wcRec := range wcResp.BriefRecords {
		groupRec := v4api.Group{Value: wcRec.OclcNumber, Count: 1}
		groupRec.Records = make([]v4api.Record, 0)
		record := v4api.Record{}
		record.Fields = getResultFields(wcRec)
		groupRec.Records = append(groupRec.Records, record)
		v4Resp.Groups = append(v4Resp.Groups, groupRec)
	}

	v4Resp.StatusCode = http.StatusOK
	c.JSON(http.StatusOK, v4Resp)
}

// Facets placeholder implementaion for a V4 facet POST.
func (svc *ServiceContext) facets(c *gin.Context) {
	log.Printf("Facets requested, but WorldCat does not support this")
	empty := make(map[string]any)
	empty["facets"] = make([]v4api.Facet, 0)
	c.JSON(http.StatusOK, empty)
}

// GetResource will get a WorkdCat resource by ID
func (svc *ServiceContext) getResource(c *gin.Context) {
	id := c.Param("id")
	log.Printf("Resource %s details requested", id)
	qURL := fmt.Sprintf("%s/worldcat/search/v2/bibs/%s", svc.OCLC.API, id)
	rawResp, respErr := svc.apiGet(qURL, svc.OCLC.Token)
	if respErr != nil {
		c.String(respErr.StatusCode, respErr.Message)
		return
	}

	wcResp := wcDetailRecord{}
	fmtErr := json.Unmarshal(rawResp, &wcResp)
	if fmtErr != nil {
		log.Printf("ERROR: Invalid response from WorldCat API: %s", fmtErr.Error())
		log.Printf("Response: %s", rawResp)
		c.String(http.StatusInternalServerError, fmtErr.Error())
		return
	}

	var jsonResp struct {
		Fields []v4api.RecordField `json:"fields"`
	}
	jsonResp.Fields = getDetailFields(wcResp)

	c.JSON(http.StatusOK, jsonResp)
}

func (svc *ServiceContext) refreshOCLCAuth() error {
	log.Printf("INFO: check OCLC auth token")
	now := time.Now()
	del := svc.OCLC.Expires.Sub(now)
	log.Printf("INFO: token expire [%s] vs time now [%s] : delta [%d] secs", svc.OCLC.Expires.String(), now.String(), int(del.Seconds()))
	if del.Seconds() < 0 {
		log.Printf("INFO: token is expired; requesting new OCLC auth token")
		err := svc.oclcTokenRequest()
		if err != nil {
			return errors.New(err.Message)
		}
		log.Printf("INFO: oclc auth successfully updated")
	} else {
		log.Printf("INFO: oclc auth is not expired")
	}
	return nil
}

func convertDateCriteria(query string) (string, error) {
	// DATES: date: {1987} OR date: {AFTER 2010} OR date: {BEFORE 1990} OR date: {1987 TO 1990}
	for true {
		dateIdx := strings.Index(query, "date: ")
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
			qt = fmt.Sprintf("yr:%s-", year)
		} else if strings.Contains(qt, "BEFORE") {
			yearStr := strings.Trim(strings.ReplaceAll(qt, "BEFORE", ""), " ")
			year, err := extractYear(yearStr)
			if err != nil {
				return "", err
			}
			qt = fmt.Sprintf("yr:-%s", year)
		} else if strings.Contains(qt, "TO") {
			years := strings.Split(qt, " TO ")
			yearFrom, err := extractYear(years[0])
			if err != nil {
				return "", errors.New("Starting year is invalid")
			}
			yearTo, err := extractYear(years[1])
			if err != nil {
				return "", errors.New("Ending year is invalid")
			}
			qt = fmt.Sprintf("yr:%s-%s", yearFrom, yearTo)
		} else {
			yearStr := strings.Trim(qt, " ")
			year, err := extractYear(yearStr)
			if err != nil {
				return "", err
			}
			qt = "yr:" + year
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
	if sort.SortID == v4api.SortTitle.String() {
		return "title"
	}
	if sort.SortID == v4api.SortDate.String() {
		if sort.Order == "asc" {
			return "publicationDateAsc"
		}
		return "publicationDateDesc"
	}
	return "bestMatch"
}

func getResultFields(wcRec wcBriefRecord) []v4api.RecordField {
	fields := make([]v4api.RecordField, 0)
	f := v4api.RecordField{Name: "id", Type: "identifier", Label: "Identifier", Value: wcRec.OclcNumber}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "publication_date", Type: "publication_date", Label: "Publication Date", Value: wcRec.Date}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "language", Type: "language", Label: "Language", Value: wcRec.Language, Visibility: "detailed"}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "title", Type: "title", Label: "Title", Value: wcRec.Title}
	fields = append(fields, f)

	for _, val := range wcRec.Isbns {
		f = v4api.RecordField{Name: "isbn", Type: "isbn", Label: "ISBN", Value: val}
		fields = append(fields, f)
	}

	f = v4api.RecordField{Name: "author", Type: "author", Label: "Author", Value: html.UnescapeString(wcRec.Creator)}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "publisher", Label: "Publisher", Visibility: "detailed", Value: wcRec.Publisher}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "general_format", Label: "General Format", Type: "format", Value: wcRec.GeneralFormat}
	fields = append(fields, f)
	f = v4api.RecordField{Name: "specific_format", Label: "Specific Format", Type: "format", Value: wcRec.SpecificFormat}
	fields = append(fields, f)

	return fields
}

func getDetailFields(details wcDetailRecord) []v4api.RecordField {
	fields := make([]v4api.RecordField, 0)
	f := v4api.RecordField{Name: "id", Type: "identifier", Label: "Identifier", Value: details.Identifier.OclcNumber, CitationPart: "id"}
	fields = append(fields, f)

	title := details.Title.MainTitles[0].Text
	title = strings.Split(title, "/")[0]
	f = v4api.RecordField{Name: "title", Type: "title", Label: "Title", Value: title, CitationPart: "title"}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "author", Label: "Author", Value: details.Contributor.StatementOfResponsibility.Text, CitationPart: "author"}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "format", Label: "Format", Separator: "; ", Value: details.Format.GeneralFormat}
	fields = append(fields, f)
	f = v4api.RecordField{Name: "format", Label: "Format", Separator: "; ", Value: details.Format.SpecificFormat}
	fields = append(fields, f)

	if details.Description.Summaries != nil {
		f = v4api.RecordField{Name: "subject_summary", Label: "Summary", Value: details.Description.Summaries[0].Text, CitationPart: "abstract"}
		fields = append(fields, f)
	}

	for _, sub := range details.Subjects {
		f = v4api.RecordField{Name: "subject", Type: "subject", Label: "Subject", Value: sub.SubjectName.Text, CitationPart: "subject"}
		fields = append(fields, f)
	}

	f = v4api.RecordField{Name: "published_date", Label: "Publication Date", Value: details.Date.PublicationDate, CitationPart: "published_date"}
	fields = append(fields, f)

	f = v4api.RecordField{Name: "language", Label: "Language", Value: details.Language.ItemLanguage, CitationPart: "language"}
	fields = append(fields, f)

	if details.Title.SeriesTitles != nil {
		f = v4api.RecordField{Name: "series", Label: "Series", Value: details.Title.SeriesTitles[0].SeriesTitle}
		fields = append(fields, f)
	}

	for _, val := range details.Identifier.Isbns {
		f = v4api.RecordField{Name: "isbn", Type: "isbn", Label: "ISBN", Value: val, CitationPart: "serial_number"}
		fields = append(fields, f)
	}

	f = v4api.RecordField{Name: "description", Label: "Description", Value: details.Description.PhysicalDescription}
	fields = append(fields, f)

	for _, note := range details.Note.GeneralNotes {
		f = v4api.RecordField{Name: "notes", Label: "Notes", Separator: "paragraph", Value: note.Text}
		fields = append(fields, f)
	}

	if details.Publishers != nil {
		f = v4api.RecordField{Name: "publisher_name", Label: "Publisher", Value: details.Publishers[0].PublisherName.Text, CitationPart: "publisher"}
		fields = append(fields, f)
		f = v4api.RecordField{Name: "published_location", Label: "Publication Place", Value: details.Publishers[0].PublicationPlace, CitationPart: "published_location"}
		fields = append(fields, f)
	}

	f = v4api.RecordField{Name: "worldcat_url", Type: "url", Label: "View full metadata on WorldCat",
		Value: fmt.Sprintf("http://worldcat.org/oclc/%s", details.Identifier.OclcNumber)}
	fields = append(fields, f)

	return fields
}
