package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
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

// Parse reads the command-line flags. It prints the version and exits when
// -version or -v is given.
func Parse(version string) *Config {
	cfg, showVersion, err := parseArgs(os.Args[0], os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		os.Exit(0)
	}
	if err != nil {
		os.Exit(2)
	}
	if showVersion {
		fmt.Printf("lazyredis %s\n", version)
		os.Exit(0)
	}
	return cfg
}

// flagOutput is where usage and parse errors are written. nil means stderr.
// Tests set it to io.Discard.
var flagOutput io.Writer

// parseArgs parses args into a Config. It does not exit, so tests can call it.
func parseArgs(name string, args []string) (*Config, bool, error) {
	cfg := &Config{}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	if flagOutput != nil {
		fs.SetOutput(flagOutput)
	}

	var showVersion bool
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.BoolVar(&showVersion, "v", false, "print version and exit")

	fs.StringVar(&cfg.Host, "host", "127.0.0.1", "Redis host")
	fs.IntVar(&cfg.Port, "port", 6379, "Redis port")
	fs.StringVar(&cfg.Password, "pass", "", "Redis password")
	fs.IntVar(&cfg.DB, "db", 0, "Redis database number (0-15)")
	fs.BoolVar(&cfg.TLS, "tls", false, "Enable TLS/SSL")
	fs.BoolVar(&cfg.TLSSkipVerify, "tls-skip-verify", false, "Skip TLS certificate verification (insecure)")
	fs.StringVar(&cfg.TLSCert, "tls-cert", "", "Path to TLS client certificate file")
	fs.StringVar(&cfg.TLSKey, "tls-key", "", "Path to TLS client key file")
	fs.StringVar(&cfg.TLSCA, "tls-ca", "", "Path to TLS CA certificate file")

	if err := fs.Parse(args); err != nil {
		return nil, false, err
	}
	return cfg, showVersion, nil
}
