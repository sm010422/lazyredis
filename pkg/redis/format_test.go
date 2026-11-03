package redis

import (
	"testing"
	"time"
)

func TestIsJSON(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{`{"a":1}`, true},
		{`  [1, 2, 3]  `, true},
		{"{\n  \"nested\": {\"b\": [true, null]}\n}", true},
		{`{"a":`, false},
		{`"just a string"`, false}, // valid JSON, but not worth pretty-printing
		{`123`, false},
		{`{not json}`, false},
		{``, false},
		{`   `, false},
	}
	for _, tt := range tests {
		if got := IsJSON(tt.in); got != tt.want {
			t.Errorf("IsJSON(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024*1024 - 1, "1024.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{5 * 1024 * 1024 * 1024, "5120.0 MB"},
	}
	for _, tt := range tests {
		if got := FormatSize(tt.in); got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatTTL(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		// go-redis v9 returns TTL -2 (missing key) and -1 (no expiry) as raw
		// nanosecond values, not seconds.
		{-2, "expired"},
		{-1, "persistent"},
		{0, "0s"},
		{59 * time.Second, "59s"},
		{time.Minute, "1m0s"},
		{90 * time.Second, "1m30s"},
		{time.Hour, "1h0m"},
		{25*time.Hour + 5*time.Minute, "25h5m"},
	}
	for _, tt := range tests {
		if got := FormatTTL(tt.in); got != tt.want {
			t.Errorf("FormatTTL(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0h 0m"},
		{59, "0h 0m"},
		{3661, "1h 1m"},
		{86400, "1d 0h 0m"},
		{2*86400 + 3*3600 + 4*60, "2d 3h 4m"},
	}
	for _, tt := range tests {
		if got := FormatUptime(tt.in); got != tt.want {
			t.Errorf("FormatUptime(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseServerInfo(t *testing.T) {
	// Trimmed INFO reply from Redis 7.2, with the \r\n line endings Redis sends.
	raw := "# Server\r\n" +
		"redis_version:7.2.4\r\n" +
		"redis_mode:standalone\r\n" +
		"os:Linux 6.5.0 aarch64\r\n" +
		"arch_bits:64\r\n" +
		"uptime_in_seconds:93784\r\n" +
		"\r\n" +
		"# Clients\r\n" +
		"connected_clients:3\r\n" +
		"\r\n" +
		"# Memory\r\n" +
		"used_memory:1065184\r\n" +
		"used_memory_human:1.02M\r\n" +
		"\r\n" +
		"# Stats\r\n" +
		"total_commands_processed:1200\r\n" +
		"keyspace_hits:900\r\n" +
		"keyspace_misses:100\r\n" +
		"pubsub_channels:2\r\n" +
		"pubsub_patterns:1\r\n" +
		"\r\n" +
		"# Replication\r\n" +
		"role:master\r\n" +
		"\r\n" +
		"# Keyspace\r\n" +
		"db0:keys=42,expires=1,avg_ttl=0\r\n" +
		"line-without-colon\r\n"

	got := parseServerInfo(raw)
	want := ServerInfo{
		Version:          "7.2.4",
		Mode:             "standalone",
		OS:               "Linux 6.5.0 aarch64",
		Arch:             "64-bit",
		UptimeSecs:       93784,
		ConnectedClients: 3,
		UsedMemory:       "1.02M",
		KeyspaceHits:     900,
		KeyspaceMisses:   100,
		TotalCommands:    1200,
		Role:             "master",
		PubSubChannels:   2,
		PubSubPatterns:   1,
	}
	if *got != want {
		t.Errorf("parseServerInfo:\n got  %+v\n want %+v", *got, want)
	}
}

func TestParseServerInfoIgnoresBadNumbers(t *testing.T) {
	got := parseServerInfo("connected_clients:lots\nredis_version:7.0.0\n")
	if got.ConnectedClients != 0 || got.Version != "7.0.0" {
		t.Errorf("got %+v, want ConnectedClients=0 and Version=7.0.0", *got)
	}
}
