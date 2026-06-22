package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Profile struct {
	Name          string `json:"name"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	Password      string `json:"password,omitempty"`
	DB            int    `json:"db"`
	TLS           bool   `json:"tls,omitempty"`
	TLSSkipVerify bool   `json:"tls_skip_verify,omitempty"`
	TLSCert       string `json:"tls_cert,omitempty"`
	TLSKey        string `json:"tls_key,omitempty"`
	TLSCA         string `json:"tls_ca,omitempty"`
	Color         string `json:"color,omitempty"` // "green","blue","red","yellow","purple","peach","teal","pink"
}

func (p *Profile) ToConfig() *Config {
	return &Config{
		Host:          p.Host,
		Port:          p.Port,
		Password:      p.Password,
		DB:            p.DB,
		TLS:           p.TLS,
		TLSSkipVerify: p.TLSSkipVerify,
		TLSCert:       p.TLSCert,
		TLSKey:        p.TLSKey,
		TLSCA:         p.TLSCA,
	}
}

type profileFile struct {
	Profiles []Profile `json:"profiles"`
}

func ProfilesPath() string {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "lazyredis", "config.json")
}

func LoadProfiles() ([]Profile, error) {
	data, err := os.ReadFile(ProfilesPath())
	if os.IsNotExist(err) {
		ps := defaultProfiles()
		_ = SaveProfiles(ps)
		return ps, nil
	}
	if err != nil {
		return nil, err
	}
	var f profileFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	if len(f.Profiles) == 0 {
		return defaultProfiles(), nil
	}
	return f.Profiles, nil
}

func SaveProfiles(profiles []Profile) error {
	path := ProfilesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(profileFile{Profiles: profiles}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func defaultProfiles() []Profile {
	return []Profile{
		{Name: "local", Host: "127.0.0.1", Port: 6379, DB: 0, Color: "green"},
	}
}
