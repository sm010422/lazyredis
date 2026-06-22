package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/parksangmin/lazyredis/pkg/config"
	goredis "github.com/redis/go-redis/v9"
)

// noopLogger silences go-redis internal connection pool error logs so they
// don't flood the terminal when Redis is unreachable.
type noopLogger struct{}

func (noopLogger) Printf(_ context.Context, _ string, _ ...interface{}) {}

func init() {
	goredis.SetLogger(noopLogger{})
}

type KeyType string

const (
	TypeString KeyType = "string"
	TypeList   KeyType = "list"
	TypeSet    KeyType = "set"
	TypeZSet   KeyType = "zset"
	TypeHash   KeyType = "hash"
	TypeStream KeyType = "stream"
	TypeNone   KeyType = "none"
)

type KeyInfo struct {
	Name    string
	Type    KeyType
	TTL     time.Duration
	Size    int64
	Memory  int64
}

type ZSetMember struct {
	Member string
	Score  float64
}

type StreamEntry struct {
	ID     string
	Fields map[string]interface{}
}

type ServerInfo struct {
	Version        string
	Mode           string
	OS             string
	Arch           string
	UptimeSecs     int64
	ConnectedClients int64
	UsedMemory     string
	TotalKeys      int64
	KeyspaceHits   int64
	KeyspaceMisses int64
	TotalCommands  int64
	Role           string
}

type Client struct {
	rdb *goredis.Client
	ctx context.Context
}

func New(cfg *config.Config) (*Client, error) {
	opts := &goredis.Options{
		Addr:        cfg.Addr(),
		Password:    cfg.Password,
		DB:          cfg.DB,
		DialTimeout: 3 * time.Second,
	}

	if cfg.TLS {
		tlsCfg := &tls.Config{
			InsecureSkipVerify: cfg.TLSSkipVerify,
			ServerName:         cfg.Host,
		}
		if cfg.TLSCert != "" && cfg.TLSKey != "" {
			cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
			if err != nil {
				return nil, fmt.Errorf("load TLS cert/key: %w", err)
			}
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
		if cfg.TLSCA != "" {
			pem, err := os.ReadFile(cfg.TLSCA)
			if err != nil {
				return nil, fmt.Errorf("read TLS CA: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("failed to parse CA certificate")
			}
			tlsCfg.RootCAs = pool
		}
		opts.TLSConfig = tlsCfg
	}

	return &Client{rdb: goredis.NewClient(opts), ctx: context.Background()}, nil
}

func (c *Client) Ping() error {
	ctx, cancel := context.WithTimeout(c.ctx, 2*time.Second)
	defer cancel()
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Close() { c.rdb.Close() }

func (c *Client) SelectDB(db int) error {
	return c.rdb.Do(c.ctx, "SELECT", db).Err()
}

// GetKeyTypes returns key→type for all given keys using a single pipeline.
// TYPE only — much cheaper than GetKeyInfo when you just need the badge.
func (c *Client) GetKeyTypes(keys []string) map[string]string {
	if len(keys) == 0 {
		return nil
	}
	pipe := c.rdb.Pipeline()
	cmds := make([]*goredis.StatusCmd, len(keys))
	for i, k := range keys {
		cmds[i] = pipe.Type(c.ctx, k)
	}
	pipe.Exec(c.ctx) //nolint:errcheck — individual cmd errors handled below
	result := make(map[string]string, len(keys))
	for i, k := range keys {
		if cmds[i].Err() == nil {
			result[k] = cmds[i].Val()
		}
	}
	return result
}

// Scan uses SCAN cursor iteration to avoid blocking on large DBs.
func (c *Client) Scan(pattern string) ([]string, error) {
	if pattern == "" {
		pattern = "*"
	}
	var keys []string
	var cursor uint64
	for {
		batch, next, err := c.rdb.Scan(c.ctx, cursor, pattern, 1000).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, batch...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (c *Client) GetKeyInfo(key string) (*KeyInfo, error) {
	typ, err := c.rdb.Type(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}
	ttl, err := c.rdb.TTL(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}

	info := &KeyInfo{Name: key, Type: KeyType(typ), TTL: ttl}

	switch KeyType(typ) {
	case TypeString:
		info.Size, _ = c.rdb.StrLen(c.ctx, key).Result()
	case TypeList:
		info.Size, _ = c.rdb.LLen(c.ctx, key).Result()
	case TypeSet:
		info.Size, _ = c.rdb.SCard(c.ctx, key).Result()
	case TypeZSet:
		info.Size, _ = c.rdb.ZCard(c.ctx, key).Result()
	case TypeHash:
		info.Size, _ = c.rdb.HLen(c.ctx, key).Result()
	}

	// MEMORY USAGE (Redis >= 4.0)
	mem, err := c.rdb.MemoryUsage(c.ctx, key).Result()
	if err == nil {
		info.Memory = mem
	}

	return info, nil
}

// ---- String ----

func (c *Client) GetString(key string) (string, error) {
	return c.rdb.Get(c.ctx, key).Result()
}

func (c *Client) SetString(key, value string, ttl time.Duration) error {
	return c.rdb.Set(c.ctx, key, value, ttl).Err()
}

// ---- List ----

func (c *Client) GetList(key string, start, stop int64) ([]string, error) {
	return c.rdb.LRange(c.ctx, key, start, stop).Result()
}

func (c *Client) LPush(key string, values ...string) error {
	ivals := make([]interface{}, len(values))
	for i, v := range values {
		ivals[i] = v
	}
	return c.rdb.LPush(c.ctx, key, ivals...).Err()
}

func (c *Client) RPush(key string, values ...string) error {
	ivals := make([]interface{}, len(values))
	for i, v := range values {
		ivals[i] = v
	}
	return c.rdb.RPush(c.ctx, key, ivals...).Err()
}

func (c *Client) LSet(key string, index int64, value string) error {
	return c.rdb.LSet(c.ctx, key, index, value).Err()
}

func (c *Client) LRem(key string, count int64, value string) error {
	return c.rdb.LRem(c.ctx, key, count, value).Err()
}

func (c *Client) LPop(key string) (string, error) {
	return c.rdb.LPop(c.ctx, key).Result()
}

func (c *Client) RPop(key string) (string, error) {
	return c.rdb.RPop(c.ctx, key).Result()
}

// ---- Hash ----

func (c *Client) GetHash(key string) (map[string]string, error) {
	return c.rdb.HGetAll(c.ctx, key).Result()
}

func (c *Client) HSet(key, field, value string) error {
	return c.rdb.HSet(c.ctx, key, field, value).Err()
}

func (c *Client) HDel(key string, fields ...string) error {
	return c.rdb.HDel(c.ctx, key, fields...).Err()
}

func (c *Client) HGet(key, field string) (string, error) {
	return c.rdb.HGet(c.ctx, key, field).Result()
}

// ---- Set ----

func (c *Client) GetSet(key string) ([]string, error) {
	members, err := c.rdb.SMembers(c.ctx, key).Result()
	if err != nil {
		return nil, err
	}
	sort.Strings(members)
	return members, nil
}

func (c *Client) SAdd(key string, members ...string) error {
	ivals := make([]interface{}, len(members))
	for i, v := range members {
		ivals[i] = v
	}
	return c.rdb.SAdd(c.ctx, key, ivals...).Err()
}

func (c *Client) SRem(key string, members ...string) error {
	ivals := make([]interface{}, len(members))
	for i, v := range members {
		ivals[i] = v
	}
	return c.rdb.SRem(c.ctx, key, ivals...).Err()
}

// ---- ZSet ----

func (c *Client) GetZSet(key string, start, stop int64) ([]goredis.Z, error) {
	return c.rdb.ZRangeWithScores(c.ctx, key, start, stop).Result()
}

func (c *Client) ZAdd(key, member string, score float64) error {
	return c.rdb.ZAdd(c.ctx, key, goredis.Z{Score: score, Member: member}).Err()
}

func (c *Client) ZRem(key string, members ...string) error {
	ivals := make([]interface{}, len(members))
	for i, v := range members {
		ivals[i] = v
	}
	return c.rdb.ZRem(c.ctx, key, ivals...).Err()
}

func (c *Client) ZIncrBy(key string, increment float64, member string) error {
	return c.rdb.ZIncrBy(c.ctx, key, increment, member).Err()
}

// ---- Stream ----

func (c *Client) GetStream(key string, count int64) ([]StreamEntry, error) {
	entries, err := c.rdb.XRevRangeN(c.ctx, key, "+", "-", count).Result()
	if err != nil {
		return nil, err
	}
	result := make([]StreamEntry, len(entries))
	for i, e := range entries {
		result[i] = StreamEntry{ID: e.ID, Fields: e.Values}
	}
	return result, nil
}

func (c *Client) XAdd(key string, fields map[string]string) (string, error) {
	values := make([]string, 0, len(fields)*2)
	for k, v := range fields {
		values = append(values, k, v)
	}
	ivals := make([]interface{}, len(values))
	for i, v := range values {
		ivals[i] = v
	}
	return c.rdb.XAdd(c.ctx, &goredis.XAddArgs{
		Stream: key,
		Values: ivals,
	}).Result()
}

// ---- Key operations ----

func (c *Client) Delete(keys ...string) (int64, error) {
	return c.rdb.Del(c.ctx, keys...).Result()
}

// Unlink asynchronously deletes keys (non-blocking). Falls back to DEL if unavailable.
func (c *Client) Unlink(keys ...string) (int64, error) {
	n, err := c.rdb.Unlink(c.ctx, keys...).Result()
	if err != nil {
		return c.Delete(keys...)
	}
	return n, nil
}

// BatchUnlink deletes keys in chunks to avoid blocking Redis.
func (c *Client) BatchUnlink(keys []string) error {
	const chunkSize = 500
	for i := 0; i < len(keys); i += chunkSize {
		end := i + chunkSize
		if end > len(keys) {
			end = len(keys)
		}
		if _, err := c.Unlink(keys[i:end]...); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) Rename(oldKey, newKey string) error {
	return c.rdb.Rename(c.ctx, oldKey, newKey).Err()
}

func (c *Client) Expire(key string, ttl time.Duration) error {
	return c.rdb.Expire(c.ctx, key, ttl).Err()
}

func (c *Client) Persist(key string) error {
	return c.rdb.Persist(c.ctx, key).Err()
}

func (c *Client) Exists(key string) (bool, error) {
	n, err := c.rdb.Exists(c.ctx, key).Result()
	return n > 0, err
}

func (c *Client) Copy(src, dst string) error {
	return c.rdb.Copy(c.ctx, src, dst, 0, false).Err()
}

// ---- DB / Server ----

func (c *Client) DBSize() (int64, error) {
	return c.rdb.DBSize(c.ctx).Result()
}

func (c *Client) FlushDB() error {
	return c.rdb.FlushDB(c.ctx).Err()
}

func (c *Client) GetServerInfo() (*ServerInfo, error) {
	raw, err := c.rdb.Info(c.ctx).Result()
	if err != nil {
		return nil, err
	}
	info := &ServerInfo{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		switch k {
		case "redis_version":
			info.Version = v
		case "redis_mode":
			info.Mode = v
		case "os":
			info.OS = v
		case "arch_bits":
			info.Arch = v + "-bit"
		case "uptime_in_seconds":
			info.UptimeSecs, _ = strconv.ParseInt(v, 10, 64)
		case "connected_clients":
			info.ConnectedClients, _ = strconv.ParseInt(v, 10, 64)
		case "used_memory_human":
			info.UsedMemory = v
		case "total_commands_processed":
			info.TotalCommands, _ = strconv.ParseInt(v, 10, 64)
		case "keyspace_hits":
			info.KeyspaceHits, _ = strconv.ParseInt(v, 10, 64)
		case "keyspace_misses":
			info.KeyspaceMisses, _ = strconv.ParseInt(v, 10, 64)
		case "role":
			info.Role = v
		}
	}
	return info, nil
}

func (c *Client) GetRawInfo(section string) (string, error) {
	if section == "" {
		return c.rdb.Info(c.ctx).Result()
	}
	return c.rdb.Info(c.ctx, section).Result()
}

// GetModuleValue fetches the value of a Redis module key (JSON, TimeSeries, etc.)
// using the appropriate module command where possible, otherwise falls back to a hint.
func (c *Client) GetModuleValue(key, typ string) (string, error) {
	switch typ {
	case "ReJSON-RL", "json":
		raw, err := c.rdb.Do(c.ctx, "JSON.GET", key).Text()
		if err != nil {
			return "", err
		}
		return raw, nil
	case "TSDB-TYPE":
		raw, err := c.rdb.Do(c.ctx, "TS.INFO", key).Result()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%v", raw), nil
	case "vectorset":
		raw, err := c.rdb.Do(c.ctx, "VCARD", key).Result()
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Vector Set  (%v elements)\n\nUse : VRANDMEMBER %s to inspect members.", raw, key), nil
	default:
		return "", fmt.Errorf("unsupported module type: %s", typ)
	}
}

// Do executes a raw Redis command.
func (c *Client) Do(args ...interface{}) (interface{}, error) {
	return c.rdb.Do(c.ctx, args...).Result()
}

// ---- Helpers ----

func IsJSON(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	return (s[0] == '{' || s[0] == '[') && json.Valid([]byte(s))
}

func FormatSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
	}
}

func FormatTTL(d time.Duration) string {
	switch {
	case d == -2:
		return "expired"
	case d == -1:
		return "persistent"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		return fmt.Sprintf("%dh%dm", h, m)
	}
}

func FormatUptime(secs int64) string {
	days := secs / 86400
	hours := (secs % 86400) / 3600
	mins := (secs % 3600) / 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, mins)
	}
	return fmt.Sprintf("%dh %dm", hours, mins)
}
