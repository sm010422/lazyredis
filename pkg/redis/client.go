package redis

import (
	"context"
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
}

type HashField struct {
	Field string
	Value string
}

type ZSetMember struct {
	Member string
	Score  float64
}

type Client struct {
	rdb  *goredis.Client
	ctx  context.Context
}

func New(cfg *config.Config) *Client {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	return &Client{
		rdb: rdb,
		ctx: context.Background(),
	}
}

func (c *Client) Ping() error {
	return c.rdb.Ping(c.ctx).Err()
}

func (c *Client) Close() {
	c.rdb.Close()
}

func (c *Client) Keys(pattern string) ([]string, error) {
	if pattern == "" {
		pattern = "*"
	}
	keys, err := c.rdb.Keys(c.ctx, pattern).Result()
	if err != nil {
		return nil, err
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

	info := &KeyInfo{
		Name: key,
		Type: KeyType(typ),
		TTL:  ttl,
	}

	switch KeyType(typ) {
	case TypeString:
		n, _ := c.rdb.StrLen(c.ctx, key).Result()
		info.Size = n
	case TypeList:
		n, _ := c.rdb.LLen(c.ctx, key).Result()
		info.Size = n
	case TypeSet:
		n, _ := c.rdb.SCard(c.ctx, key).Result()
		info.Size = n
	case TypeZSet:
		n, _ := c.rdb.ZCard(c.ctx, key).Result()
		info.Size = n
	case TypeHash:
		n, _ := c.rdb.HLen(c.ctx, key).Result()
		info.Size = n
	}

	return info, nil
}

func (c *Client) GetValue(key string) (string, error) {
	typ, err := c.rdb.Type(c.ctx, key).Result()
	if err != nil {
		return "", err
	}

	switch KeyType(typ) {
	case TypeString:
		return c.rdb.Get(c.ctx, key).Result()

	case TypeList:
		vals, err := c.rdb.LRange(c.ctx, key, 0, 99).Result()
		if err != nil {
			return "", err
		}
		lines := make([]string, len(vals))
		for i, v := range vals {
			lines[i] = fmt.Sprintf("%3d) %q", i, v)
		}
		return strings.Join(lines, "\n"), nil

	case TypeSet:
		vals, err := c.rdb.SMembers(c.ctx, key).Result()
		if err != nil {
			return "", err
		}
		sort.Strings(vals)
		lines := make([]string, len(vals))
		for i, v := range vals {
			lines[i] = fmt.Sprintf("%3d) %q", i+1, v)
		}
		return strings.Join(lines, "\n"), nil

	case TypeZSet:
		vals, err := c.rdb.ZRangeWithScores(c.ctx, key, 0, 99).Result()
		if err != nil {
			return "", err
		}
		lines := make([]string, len(vals))
		for i, z := range vals {
			lines[i] = fmt.Sprintf("%3d) score=%-10s  %q", i+1, strconv.FormatFloat(z.Score, 'f', -1, 64), z.Member)
		}
		return strings.Join(lines, "\n"), nil

	case TypeHash:
		fields, err := c.rdb.HGetAll(c.ctx, key).Result()
		if err != nil {
			return "", err
		}
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		lines := make([]string, len(keys))
		for i, k := range keys {
			lines[i] = fmt.Sprintf("%-20s  %q", k, fields[k])
		}
		return strings.Join(lines, "\n"), nil

	case TypeStream:
		entries, err := c.rdb.XRange(c.ctx, key, "-", "+").Result()
		if err != nil {
			return "", err
		}
		var lines []string
		for _, e := range entries {
			lines = append(lines, fmt.Sprintf("[%s]", e.ID))
			fieldKeys := make([]string, 0, len(e.Values))
			for k := range e.Values {
				fieldKeys = append(fieldKeys, k)
			}
			sort.Strings(fieldKeys)
			for _, k := range fieldKeys {
				lines = append(lines, fmt.Sprintf("  %-20s  %v", k, e.Values[k]))
			}
		}
		return strings.Join(lines, "\n"), nil
	}

	return "", fmt.Errorf("unsupported type: %s", typ)
}

func (c *Client) SetString(key, value string) error {
	return c.rdb.Set(c.ctx, key, value, 0).Err()
}

func (c *Client) Delete(keys ...string) (int64, error) {
	return c.rdb.Del(c.ctx, keys...).Result()
}

func (c *Client) Info() (string, error) {
	return c.rdb.Info(c.ctx).Result()
}

func (c *Client) DBSize() (int64, error) {
	return c.rdb.DBSize(c.ctx).Result()
}
