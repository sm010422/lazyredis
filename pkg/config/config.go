package config

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	Host          string
	Port          int
	Password      string
	DB            int
	TLS           bool
	TLSSkipVerify bool
	TLSCert       string
	TLSKey        string
	TLSCA         string
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func Parse(version string) *Config {
	cfg := &Config{}

	var showVersion bool
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&showVersion, "v", false, "print version and exit")

	flag.StringVar(&cfg.Host, "host", "127.0.0.1", "Redis host")
	flag.IntVar(&cfg.Port, "port", 6379, "Redis port")
	flag.StringVar(&cfg.Password, "pass", "", "Redis password")
	flag.IntVar(&cfg.DB, "db", 0, "Redis database number (0-15)")
	flag.BoolVar(&cfg.TLS, "tls", false, "Enable TLS/SSL")
	flag.BoolVar(&cfg.TLSSkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification (insecure)")
	flag.StringVar(&cfg.TLSCert, "tls-cert", "", "Path to TLS client certificate file")
	flag.StringVar(&cfg.TLSKey, "tls-key", "", "Path to TLS client key file")
	flag.StringVar(&cfg.TLSCA, "tls-ca", "", "Path to TLS CA certificate file")
	flag.Parse()

	if showVersion {
		fmt.Printf("lazyredis %s\n", version)
		os.Exit(0)
	}

	return cfg
}
