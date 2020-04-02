package main

// SortOptionEnum is the enumerated type for WorldCat sort options
type SortOptionEnum int

const (
	// SortRelevance is used to sort by descending relevance
	SortRelevance SortOptionEnum = iota
	// SortDate is used to sort by published date
	SortDate
	// SortTitle is used to sort by title
	SortTitle
	// SortAuthor is used to sort by Author
	SortAuthor
)

func (r SortOptionEnum) String() string {
	return []string{"SortRelevance", "SortDatePublished", "SortTitle", "SortAuthor"}[r]
}

// PoolAttribute describes a capability of a pool
type PoolAttribute struct {
	Name      string `json:"name"`
	Supported bool   `json:"supported"`
	Value     string `json:"value,omitempty"`
}

// PoolIdentity contains the complete data that descibes a pool and its abilities
type PoolIdentity struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Mode        string          `json:"mode"`
	Attributes  []PoolAttribute `json:"attributes,omitempty"`
	SortOptions []SortOption    `json:"sort_options,omitempty"`
}

// SortOrder specifies sort ordering for a given search.
type SortOrder struct {
	SortID string `json:"sort_id"`
	Order  string `json:"order"`
}

// SortOption defines a sorting option for a pool
type SortOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// SearchRequest contains all of the data necessary for a client seatch request
// Note that the JMRL pool does not support facets/filters
type SearchRequest struct {
	Query      string     `json:"query"`
	Pagination Pagination `json:"pagination"`
	Sort       SortOrder  `json:"sort,omitempty"`
}

// Pagination cantains pagination info
type Pagination struct {
	Start int `json:"start"`
	Rows  int `json:"rows"`
	Total int `json:"total"`
}

// PoolResult is the response V4API response format
type PoolResult struct {
	ElapsedMS       int64                  `json:"elapsed_ms,omitempty"`
	Pagination      Pagination             `json:"pagination"`
	Sort            SortOrder              `json:"sort,omitempty"`
	Groups          []Group                `json:"group_list,omitempty"`
	Confidence      string                 `json:"confidence,omitempty"`
	Debug           map[string]interface{} `json:"debug"`
	Warnings        []string               `json:"warnings"`
	StatusCode      int                    `json:"status_code"`
	StatusMessage   string                 `json:"status_msg,omitempty"`
	ContentLanguage string                 `json:"-"`
}

// Group contains the records for a single group in a search result set.
type Group struct {
	Value   string   `json:"value"`
	Count   int      `json:"count"`
	Records []Record `json:"record_list,omitempty"`
}

// Record is a summary of one search hit
type Record struct {
	Fields []RecordField          `json:"fields"`
	Debug  map[string]interface{} `json:"debug"`
}

// RecordField contains metadata for a single field in a record.
type RecordField struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"` // empty implies "text"
	Label      string `json:"label"`
	Value      string `json:"value"`
	Visibility string `json:"visibility,omitempty"` // e.g. "basic" or "detailed".  empty implies "basic"
	Display    string `json:"display,omitempty"`    // e.g. "optional".  empty implies not optional
	Provider   string `json:"provider,omitempty"`
}
