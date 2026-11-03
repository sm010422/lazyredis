package config

import (
	"errors"
	"flag"
	"io"
	"testing"
)

func TestParseArgsDefaults(t *testing.T) {
	cfg, showVersion, err := parseArgs("lazyredis", nil)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if showVersion {
		t.Error("showVersion = true, want false")
	}
	want := Config{Host: "127.0.0.1", Port: 6379}
	if *cfg != want {
		t.Errorf("cfg = %+v, want %+v", *cfg, want)
	}
}

func TestParseArgsAllFlags(t *testing.T) {
	args := []string{
		"-host", "redis.internal", "-port", "6380", "-pass", "secret", "-db", "3",
		"-tls", "-tls-skip-verify",
		"-tls-cert", "/c.pem", "-tls-key", "/k.pem", "-tls-ca", "/ca.pem",
	}
	cfg, _, err := parseArgs("lazyredis", args)
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	want := Config{
		Host: "redis.internal", Port: 6380, Password: "secret", DB: 3,
		TLS: true, TLSSkipVerify: true,
		TLSCert: "/c.pem", TLSKey: "/k.pem", TLSCA: "/ca.pem",
	}
	if *cfg != want {
		t.Errorf("cfg = %+v, want %+v", *cfg, want)
	}
}

func TestParseArgsVersion(t *testing.T) {
	for _, arg := range []string{"-v", "-version", "--version"} {
		_, showVersion, err := parseArgs("lazyredis", []string{arg})
		if err != nil {
			t.Fatalf("%s: %v", arg, err)
		}
		if !showVersion {
			t.Errorf("%s: showVersion = false, want true", arg)
		}
	}
}

func TestParseArgsErrors(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{"help", []string{"-h"}, flag.ErrHelp},
		{"unknown flag", []string{"-bogus"}, nil},
		{"non-numeric port", []string{"-port", "abc"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Keep the usage text out of the test output.
			flagOutput = io.Discard
			t.Cleanup(func() { flagOutput = nil })

			_, _, err := parseArgs("lazyredis", tt.args)
			if err == nil {
				t.Fatal("err = nil, want an error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestAddr(t *testing.T) {
	tests := []struct {
		cfg  Config
		want string
	}{
		{Config{Host: "127.0.0.1", Port: 6379}, "127.0.0.1:6379"},
		{Config{Host: "redis.example.com", Port: 16379}, "redis.example.com:16379"},
	}
	for _, tt := range tests {
		if got := tt.cfg.Addr(); got != tt.want {
			t.Errorf("Addr() = %q, want %q", got, tt.want)
		}
	}
}
