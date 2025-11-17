package main

import (
	"flag"
	"log"
)

// ServiceConfig defines all of the JRML pool configuration parameters
type ServiceConfig struct {
	Port        int
	WCAPI       string
	JWTKey      string
	OCLCKey     string
	OCLCSecret  string
	OCLCAuthURL string
}

// LoadConfiguration will load the service configuration from env/cmdline
// and return a pointer to it. Any failures are fatal.
func LoadConfiguration() *ServiceConfig {
	log.Printf("Loading configuration...")
	var cfg ServiceConfig
	flag.IntVar(&cfg.Port, "port", 8080, "JRML pool service port (default 8080)")
	flag.StringVar(&cfg.WCAPI, "wcapi", "", "WorldCat API base URL")
	flag.StringVar(&cfg.JWTKey, "jwtkey", "", "JWT signature key")
	flag.StringVar(&cfg.OCLCKey, "oclckey", "", "OCLC API key")
	flag.StringVar(&cfg.OCLCSecret, "oclcsecret", "", "OCLC API secret")
	flag.StringVar(&cfg.OCLCAuthURL, "oclcauth", "https://oauth.oclc.org/token?grant_type=client_credentials&scope=wcapi:view_brief_bib%20wcapi:view_bib", "OCLC Auth endpoint")

	flag.Parse()

	if cfg.WCAPI == "" {
		log.Fatal("Parameter -wcapi is required")
	}
	if cfg.JWTKey == "" {
		log.Fatal("jwtkey param is required")
	}
	if cfg.OCLCKey == "" {
		log.Fatal("oclckey param is required")
	}
	if cfg.OCLCSecret == "" {
		log.Fatal("oclcsecret param is required")
	}

	log.Printf("[CONFIG] port          = [%d]", cfg.Port)
	log.Printf("[CONFIG] wcapi         = [%s]", cfg.WCAPI)
	log.Printf("[CONFIG] oclckey       = [%s]", cfg.OCLCKey)
	log.Printf("[CONFIG] oclcauth      = [%s]", cfg.OCLCAuthURL)

	return &cfg
}
