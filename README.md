# Virgo4 WorldCat search pool

This is the Virgo4 pool for WorldCat.
It implements the Virgo4 API using the WorldCate API detailed here: 
https://platform.worldcat.org/api-explorer/apis/wcapi

### System Requirements
* GO version 1.14 or greater (mod required)

### Current API

* GET /version : returns build version
* GET /identify : returns pool information
* GET /healthcheck : returns health check information
* GET /metrics : returns Prometheus metrics
* GET /api/providers : returns a list of link providers
* POST /api/search : returns search results
* GET /api/resource/{id} : returns detailed information for a single Solr record
