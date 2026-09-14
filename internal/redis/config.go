package redis

import "time"

const (
	DefaultBTreeDegree = 8
)

type Config struct {
	DefaultTTLSeconds int64
	BTreeDegree       int
	Now               func() time.Time
}

func defaultConfig() Config {
	return Config{
		BTreeDegree: DefaultBTreeDegree,
		Now:         time.Now,
	}
}
