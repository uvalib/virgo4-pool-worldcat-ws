package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
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
	c.JSON(http.StatusOK, p)
}

// Search accepts a search POST, transforms the query into JMRL format and perfoms the search
func (svc *ServiceContext) search(c *gin.Context) {
	log.Printf("Search requested")
	c.String(http.StatusNotImplemented, "under construction")
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
