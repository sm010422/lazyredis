package redis

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/sm010422/lazyredis/pkg/config"
)

// testRedisAddrEnv selects a real Redis server for these tests, for example
// LAZYREDIS_TEST_REDIS_ADDR=127.0.0.1:6379. The tests use DB 15 and flush it.
// When unset, each test gets its own in-process miniredis.
const testRedisAddrEnv = "LAZYREDIS_TEST_REDIS_ADDR"

const testDB = 15

func newTestClient(t *testing.T) *Client {
	t.Helper()

	addr := os.Getenv(testRedisAddrEnv)
	if addr == "" {
		addr = miniredis.RunT(t).Addr()
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("bad %s %q: %v", testRedisAddrEnv, addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("bad port in %q: %v", addr, err)
	}

	c, err := New(&config.Config{Host: host, Port: port, DB: testDB})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.Ping(); err != nil {
		t.Fatalf("Ping %s: %v", addr, err)
	}
	if err := c.FlushDB(); err != nil {
		t.Fatalf("FlushDB: %v", err)
	}
	t.Cleanup(func() {
		_ = c.FlushDB()
		c.Close()
	})
	return c
}

func usingRealRedis() bool { return os.Getenv(testRedisAddrEnv) != "" }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestScanReturnsSortedKeysAcrossCursorPages(t *testing.T) {
	c := newTestClient(t)

	// More than one SCAN page (COUNT is 1000) so cursor iteration is exercised.
	const n = 2500
	want := make([]string, 0, n)
	for i := 0; i < n; i++ {
		k := fmt.Sprintf("user:%04d", i)
		want = append(want, k)
		must(t, c.SetString(k, "v", 0))
	}

	got, err := c.Scan("")
	must(t, err)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan returned %d keys, want %d sorted keys", len(got), len(want))
	}
}

func TestScanPattern(t *testing.T) {
	c := newTestClient(t)
	for _, k := range []string{"session:b", "session:a", "user:1", "세션:한글"} {
		must(t, c.SetString(k, "v", 0))
	}

	got, err := c.Scan("session:*")
	must(t, err)
	if want := []string{"session:a", "session:b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Scan(session:*) = %v, want %v", got, want)
	}

	got, err = c.Scan("세션:*")
	must(t, err)
	if want := []string{"세션:한글"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Scan(세션:*) = %v, want %v", got, want)
	}

	got, err = c.Scan("nomatch:*")
	must(t, err)
	if len(got) != 0 {
		t.Errorf("Scan(nomatch:*) = %v, want none", got)
	}
}

func TestGetKeyTypes(t *testing.T) {
	c := newTestClient(t)
	must(t, c.SetString("s", "v", 0))
	must(t, c.RPush("l", "a"))
	must(t, c.HSet("h", "f", "v"))
	must(t, c.SAdd("st", "m"))
	must(t, c.ZAdd("z", "m", 1))
	_, err := c.XAdd("x", map[string]string{"f": "v"})
	must(t, err)

	got := c.GetKeyTypes([]string{"s", "l", "h", "st", "z", "x", "missing"})
	want := map[string]string{
		"s": "string", "l": "list", "h": "hash", "st": "set", "z": "zset", "x": "stream",
		"missing": "none",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetKeyTypes = %v, want %v", got, want)
	}

	if got := c.GetKeyTypes(nil); got != nil {
		t.Errorf("GetKeyTypes(nil) = %v, want nil", got)
	}
}

func TestGetKeyInfoSizeByType(t *testing.T) {
	c := newTestClient(t)
	must(t, c.SetString("s", "hello", 0))
	must(t, c.RPush("l", "a", "b", "c"))
	must(t, c.HSet("h", "f1", "v"))
	must(t, c.HSet("h", "f2", "v"))
	must(t, c.SAdd("st", "a", "b", "c", "d"))
	must(t, c.ZAdd("z", "m", 1))

	tests := []struct {
		key      string
		wantType KeyType
		wantSize int64
	}{
		{"s", TypeString, 5},
		{"l", TypeList, 3},
		{"h", TypeHash, 2},
		{"st", TypeSet, 4},
		{"z", TypeZSet, 1},
	}
	for _, tt := range tests {
		info, err := c.GetKeyInfo(tt.key)
		must(t, err)
		if info.Name != tt.key || info.Type != tt.wantType || info.Size != tt.wantSize {
			t.Errorf("GetKeyInfo(%q) = {Name:%q Type:%q Size:%d}, want {%q %q %d}",
				tt.key, info.Name, info.Type, info.Size, tt.key, tt.wantType, tt.wantSize)
		}
		if info.TTL != -1 {
			t.Errorf("GetKeyInfo(%q).TTL = %v, want -1 (persistent)", tt.key, info.TTL)
		}
		// MEMORY USAGE is only implemented by a real server.
		if usingRealRedis() && info.Memory <= 0 {
			t.Errorf("GetKeyInfo(%q).Memory = %d, want > 0", tt.key, info.Memory)
		}
	}
}

func TestGetKeyInfoMissingKey(t *testing.T) {
	c := newTestClient(t)
	info, err := c.GetKeyInfo("missing")
	must(t, err)
	if info.Type != TypeNone || info.TTL != -2 {
		t.Errorf("missing key: Type=%q TTL=%v, want none and -2", info.Type, info.TTL)
	}
	if FormatTTL(info.TTL) != "expired" {
		t.Errorf("FormatTTL(missing) = %q, want expired", FormatTTL(info.TTL))
	}
}

func TestTTLLifecycle(t *testing.T) {
	c := newTestClient(t)
	must(t, c.SetString("k", "v", 0))

	must(t, c.Expire("k", time.Hour))
	info, err := c.GetKeyInfo("k")
	must(t, err)
	if info.TTL <= 59*time.Minute || info.TTL > time.Hour {
		t.Errorf("TTL after Expire(1h) = %v", info.TTL)
	}

	must(t, c.Persist("k"))
	info, err = c.GetKeyInfo("k")
	must(t, err)
	if info.TTL != -1 {
		t.Errorf("TTL after Persist = %v, want -1", info.TTL)
	}
}

func TestStringValues(t *testing.T) {
	c := newTestClient(t)
	must(t, c.SetString("k", `{"name":"상민"}`, 0))
	got, err := c.GetString("k")
	must(t, err)
	if got != `{"name":"상민"}` {
		t.Errorf("GetString = %q", got)
	}
}

func TestListOperations(t *testing.T) {
	c := newTestClient(t)
	must(t, c.RPush("l", "b", "c"))
	must(t, c.LPush("l", "a"))
	assertList(t, c, "l", "a", "b", "c")

	must(t, c.LSet("l", 1, "B"))
	assertList(t, c, "l", "a", "B", "c")

	must(t, c.RPush("l", "a"))
	must(t, c.LRem("l", 0, "a"))
	assertList(t, c, "l", "B", "c")

	v, err := c.LPop("l")
	must(t, err)
	if v != "B" {
		t.Errorf("LPop = %q, want B", v)
	}
	v, err = c.RPop("l")
	must(t, err)
	if v != "c" {
		t.Errorf("RPop = %q, want c", v)
	}
}

func assertList(t *testing.T, c *Client, key string, want ...string) {
	t.Helper()
	got, err := c.GetList(key, 0, -1)
	must(t, err)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("list %s = %v, want %v", key, got, want)
	}
}

func TestHashOperations(t *testing.T) {
	c := newTestClient(t)
	must(t, c.HSet("h", "a", "1"))
	must(t, c.HSet("h", "b", "2"))

	v, err := c.HGet("h", "a")
	must(t, err)
	if v != "1" {
		t.Errorf("HGet = %q, want 1", v)
	}

	must(t, c.HDel("h", "a"))
	got, err := c.GetHash("h")
	must(t, err)
	if want := map[string]string{"b": "2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("GetHash = %v, want %v", got, want)
	}
}

func TestSetOperationsReturnSortedMembers(t *testing.T) {
	c := newTestClient(t)
	must(t, c.SAdd("s", "cherry", "apple", "banana"))
	must(t, c.SRem("s", "banana"))

	got, err := c.GetSet("s")
	must(t, err)
	if want := []string{"apple", "cherry"}; !reflect.DeepEqual(got, want) {
		t.Errorf("GetSet = %v, want %v", got, want)
	}
}

func TestZSetOperations(t *testing.T) {
	c := newTestClient(t)
	must(t, c.ZAdd("z", "low", 1))
	must(t, c.ZAdd("z", "high", 10))
	must(t, c.ZAdd("z", "gone", 5))
	must(t, c.ZIncrBy("z", 20, "low"))
	must(t, c.ZRem("z", "gone"))

	got, err := c.GetZSet("z", 0, -1)
	must(t, err)
	if len(got) != 2 ||
		got[0].Member != "high" || got[0].Score != 10 ||
		got[1].Member != "low" || got[1].Score != 21 {
		t.Errorf("GetZSet = %+v, want [high:10 low:21]", got)
	}
}

func TestStreamNewestFirst(t *testing.T) {
	c := newTestClient(t)
	for i := 1; i <= 3; i++ {
		_, err := c.XAdd("events", map[string]string{"n": strconv.Itoa(i)})
		must(t, err)
	}

	got, err := c.GetStream("events", 2)
	must(t, err)
	if len(got) != 2 {
		t.Fatalf("GetStream returned %d entries, want 2", len(got))
	}
	if got[0].Fields["n"] != "3" || got[1].Fields["n"] != "2" {
		t.Errorf("GetStream order = %v, %v; want newest first (3, 2)", got[0].Fields, got[1].Fields)
	}
}

func TestKeyOperations(t *testing.T) {
	c := newTestClient(t)
	must(t, c.SetString("a", "1", 0))

	must(t, c.SetString("b", "1", 0))
	must(t, c.Rename("b", "c"))

	for key, want := range map[string]bool{"a": true, "b": false, "c": true} {
		got, err := c.Exists(key)
		must(t, err)
		if got != want {
			t.Errorf("Exists(%q) = %v, want %v", key, got, want)
		}
	}

	n, err := c.Delete("a", "c", "missing")
	must(t, err)
	if n != 2 {
		t.Errorf("Delete removed %d keys, want 2", n)
	}
}

func TestBatchUnlinkDeletesAcrossChunks(t *testing.T) {
	c := newTestClient(t)

	// BatchUnlink sends 500 keys per command; 1201 keys means three chunks
	// with a partial last one.
	keys := make([]string, 1201)
	for i := range keys {
		keys[i] = fmt.Sprintf("bulk:%d", i)
		must(t, c.SetString(keys[i], "v", 0))
	}
	must(t, c.SetString("keep", "v", 0))

	must(t, c.BatchUnlink(keys))

	left, err := c.Scan("")
	must(t, err)
	if !reflect.DeepEqual(left, []string{"keep"}) {
		t.Errorf("after BatchUnlink, keys = %d left (%v...), want only [keep]", len(left), left[:min(3, len(left))])
	}

	// Nothing to delete is not an error.
	must(t, c.BatchUnlink(nil))
}

func TestSelectDBAppliesToEveryPooledConnection(t *testing.T) {
	c := newTestClient(t)
	must(t, c.SelectDB(14))
	t.Cleanup(func() {
		_ = c.FlushDB()
		_ = c.SelectDB(testDB)
	})

	// The UI refreshes in the background while the user works, so commands
	// run concurrently and use different pooled connections. Every one of
	// them must hit the selected DB.
	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- c.SetString(fmt.Sprintf("k%d", i), "v", 0)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		must(t, err)
	}

	if got, _ := c.DBSize(); got != n {
		t.Errorf("DB 14 has %d keys, want %d", got, n)
	}
	if got := c.DB(); got != 14 {
		t.Errorf("DB() = %d, want 14", got)
	}

	must(t, c.SelectDB(testDB))
	if got, _ := c.DBSize(); got != 0 {
		t.Errorf("DB %d has %d keys, want 0 (writes leaked out of DB 14)", testDB, got)
	}
}

func TestSelectDBInvalidKeepsCurrentDB(t *testing.T) {
	if !usingRealRedis() {
		t.Skip("miniredis accepts any DB index; set " + testRedisAddrEnv + " to run")
	}
	c := newTestClient(t)
	must(t, c.SetString("k", "v", 0))

	if err := c.SelectDB(9999); err == nil {
		t.Fatal("SelectDB(9999) err = nil, want an error")
	}
	if got := c.DB(); got != testDB {
		t.Errorf("DB() = %d after failed switch, want %d", got, testDB)
	}
	if ok, _ := c.Exists("k"); !ok {
		t.Error("key in the current DB is no longer visible after a failed switch")
	}
}

func TestCopyStaysInCurrentDB(t *testing.T) {
	c := newTestClient(t)
	must(t, c.SetString("src", "v", 0))
	must(t, c.Copy("src", "dst"))

	if ok, _ := c.Exists("dst"); !ok {
		t.Error("Copy did not create dst in the current DB")
	}
}

func TestDoRunsRawCommand(t *testing.T) {
	c := newTestClient(t)
	if _, err := c.Do("SET", "raw", "42"); err != nil {
		t.Fatalf("Do SET: %v", err)
	}
	got, err := c.Do("GET", "raw")
	must(t, err)
	if got != "42" {
		t.Errorf("Do GET = %v, want 42", got)
	}

	if _, err := c.Do("NOT_A_COMMAND"); err == nil {
		t.Error("Do(NOT_A_COMMAND) err = nil, want an error")
	}
}

func TestPubSub(t *testing.T) {
	c := newTestClient(t)

	// Use Channel() like the UI does. Receive() waits for the next push on a
	// RESP3 connection, so it would block here.
	sub := c.Subscribe("news")
	t.Cleanup(func() { _ = sub.Close() })
	msgs := sub.Channel()

	stats, err := c.GetPubSubStats()
	must(t, err)
	if !reflect.DeepEqual(stats.Channels, []string{"news"}) || stats.ChannelSubs["news"] != 1 {
		t.Errorf("GetPubSubStats = %+v, want news with 1 subscriber", stats)
	}

	must(t, c.Publish("news", "hello"))
	select {
	case msg := <-msgs:
		if msg.Channel != "news" || msg.Payload != "hello" {
			t.Errorf("got %s/%s, want news/hello", msg.Channel, msg.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no message received within 2s")
	}
}

func TestGetServerInfo(t *testing.T) {
	c := newTestClient(t)
	info, err := c.GetServerInfo()
	if err != nil {
		t.Fatalf("GetServerInfo: %v", err)
	}
	if usingRealRedis() && info.Version == "" {
		t.Error("Version is empty")
	}
}

func TestGetModuleValueUnsupportedType(t *testing.T) {
	c := newTestClient(t)
	_, err := c.GetModuleValue("k", "some-module-type")
	if err == nil || !strings.Contains(err.Error(), "unsupported module type") {
		t.Errorf("err = %v, want unsupported module type", err)
	}
}

func TestPingFailsWhenServerIsDown(t *testing.T) {
	m := miniredis.RunT(t)
	host, portStr, _ := net.SplitHostPort(m.Addr())
	port, _ := strconv.Atoi(portStr)
	m.Close()

	c, err := New(&config.Config{Host: host, Port: port})
	must(t, err)
	defer c.Close()
	if err := c.Ping(); err == nil {
		t.Error("Ping to a stopped server: err = nil, want an error")
	}
}

func TestNewTLSConfigErrors(t *testing.T) {
	dir := t.TempDir()
	notPEM := filepath.Join(dir, "not.pem")
	must(t, os.WriteFile(notPEM, []byte("not a certificate"), 0o600))

	tests := []struct {
		name    string
		cfg     config.Config
		wantErr string
	}{
		{
			name:    "missing client cert",
			cfg:     config.Config{TLS: true, TLSCert: filepath.Join(dir, "nope.crt"), TLSKey: filepath.Join(dir, "nope.key")},
			wantErr: "load TLS cert/key",
		},
		{
			name:    "missing CA file",
			cfg:     config.Config{TLS: true, TLSCA: filepath.Join(dir, "nope-ca.pem")},
			wantErr: "read TLS CA",
		},
		{
			name:    "CA file is not PEM",
			cfg:     config.Config{TLS: true, TLSCA: notPEM},
			wantErr: "failed to parse CA certificate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.Host, tt.cfg.Port = "127.0.0.1", 6379
			_, err := New(&tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("New err = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestNewTLSWithoutFilesSucceeds(t *testing.T) {
	c, err := New(&config.Config{Host: "127.0.0.1", Port: 6379, TLS: true, TLSSkipVerify: true})
	must(t, err)
	c.Close()
}
