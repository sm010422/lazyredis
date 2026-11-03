package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// useConfigDir points ProfilesPath at a fresh temp directory.
func useConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestProfilesPathUsesXDGConfigHome(t *testing.T) {
	dir := useConfigDir(t)
	want := filepath.Join(dir, "lazyredis", "config.json")
	if got := ProfilesPath(); got != want {
		t.Errorf("ProfilesPath() = %q, want %q", got, want)
	}
}

func TestProfilesPathFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)
	want := filepath.Join(home, ".config", "lazyredis", "config.json")
	if got := ProfilesPath(); got != want {
		t.Errorf("ProfilesPath() = %q, want %q", got, want)
	}
}

func TestLoadProfilesCreatesDefaultFile(t *testing.T) {
	useConfigDir(t)

	got, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	if !reflect.DeepEqual(got, defaultProfiles()) {
		t.Errorf("profiles = %+v, want defaults", got)
	}
	if _, err := os.Stat(ProfilesPath()); err != nil {
		t.Errorf("default config file was not written: %v", err)
	}
}

func TestSaveThenLoadRoundTrip(t *testing.T) {
	useConfigDir(t)
	want := []Profile{
		{Name: "local", Host: "127.0.0.1", Port: 6379, Color: "green"},
		{
			Name: "prod", Host: "redis.prod", Port: 6380, Password: "pw", DB: 2,
			TLS: true, TLSCA: "/etc/ca.pem", Color: "#ff0000",
		},
	}

	if err := SaveProfiles(want); err != nil {
		t.Fatalf("SaveProfiles: %v", err)
	}
	got, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip:\n got  %+v\n want %+v", got, want)
	}
}

func TestLoadProfilesEmptyListFallsBackToDefaults(t *testing.T) {
	useConfigDir(t)
	writeConfig(t, `{"profiles": []}`)

	got, err := LoadProfiles()
	if err != nil {
		t.Fatalf("LoadProfiles: %v", err)
	}
	if !reflect.DeepEqual(got, defaultProfiles()) {
		t.Errorf("profiles = %+v, want defaults", got)
	}
}

func TestLoadProfilesInvalidJSON(t *testing.T) {
	useConfigDir(t)
	writeConfig(t, `{"profiles": [`)

	if _, err := LoadProfiles(); err == nil {
		t.Fatal("LoadProfiles: err = nil, want a JSON error")
	}
	// A broken file must be left alone so the user can fix it.
	data, _ := os.ReadFile(ProfilesPath())
	if string(data) != `{"profiles": [` {
		t.Errorf("config file was overwritten: %q", data)
	}
}

func TestToConfigCopiesConnectionFields(t *testing.T) {
	p := Profile{
		Name: "x", Host: "h", Port: 1, Password: "p", DB: 4,
		TLS: true, TLSSkipVerify: true, TLSCert: "c", TLSKey: "k", TLSCA: "ca",
		Color: "red",
	}
	want := Config{
		Host: "h", Port: 1, Password: "p", DB: 4,
		TLS: true, TLSSkipVerify: true, TLSCert: "c", TLSKey: "k", TLSCA: "ca",
	}
	if got := *p.ToConfig(); got != want {
		t.Errorf("ToConfig() = %+v, want %+v", got, want)
	}
}

func TestSaveProfilesIsOwnerReadableOnly(t *testing.T) {
	useConfigDir(t)
	// A file left by an older version that wrote 0644.
	writeConfig(t, `{"profiles": []}`)

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(SaveProfiles([]Profile{{Name: "prod", Host: "h", Port: 1, Password: "secret"}}))

	info, err := os.Stat(ProfilesPath())
	must(err)
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("config file mode = %o, want 600 (it can contain passwords)", mode)
	}
}

func writeConfig(t *testing.T, content string) {
	t.Helper()
	path := ProfilesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
