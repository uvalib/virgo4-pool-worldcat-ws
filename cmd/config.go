package main

import (
	"flag"
	"log"
)

// ServiceConfig defines all of the JRML pool configuration parameters
type ServiceConfig struct {
	Port   int
	WCKey  string
	WCAPI  string
	JWTKey string
}

// LoadConfiguration will load the service configuration from env/cmdline
// and return a pointer to it. Any failures are fatal.
func LoadConfiguration() *ServiceConfig {
	log.Printf("Loading configuration...")
	var cfg ServiceConfig
	flag.IntVar(&cfg.Port, "port", 8080, "JRML pool service port (default 8080)")
	flag.StringVar(&cfg.WCAPI, "wcapi", "", "WorldCat API base URL")
	flag.StringVar(&cfg.WCKey, "wckey", "", "WordCat WSKey")
	flag.StringVar(&cfg.JWTKey, "jwtkey", "", "JWT signature key")

	flag.Parse()

	if cfg.WCAPI == "" {
		log.Fatal("Parameter -wcapi is required")
	}
	if cfg.WCKey == "" {
		log.Fatal("Parameter -wckey is required")
	}
	if cfg.JWTKey == "" {
		log.Fatal("jwtkey param is required")
	}

	return &cfg
}
