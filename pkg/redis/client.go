package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/parksangmin/lazyredis/pkg/config"
	goredis "github.com/redis/go-redis/v9"
)

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

func New(cfg *config.Config) *Client {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:        cfg.Addr(),
		Password:    cfg.Password,
		DB:          cfg.DB,
		DialTimeout: 3 * time.Second,
	})
	return &Client{rdb: rdb, ctx: context.Background()}
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
