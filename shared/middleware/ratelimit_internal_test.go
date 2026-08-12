package middleware

import (
	"fmt"
	"testing"
)

func newTestShards() []*rateLimitShard {
	shards := make([]*rateLimitShard, rateLimitShardCount)
	for i := range shards {
		shards[i] = &rateLimitShard{visitors: make(map[string]*visitor)}
	}
	return shards
}

func TestShardFor_IsDeterministic(t *testing.T) {
	shards := newTestShards()

	a := shardFor(shards, "203.0.113.5")
	b := shardFor(shards, "203.0.113.5")
	if a != b {
		t.Error("shardFor returned different shards for the same IP across two calls")
	}
}

func TestShardFor_SpreadsAcrossShards(t *testing.T) {
	shards := newTestShards()

	seen := make(map[*rateLimitShard]bool)
	for i := 0; i < 1000; i++ {
		ip := fmt.Sprintf("10.0.%d.%d", i/256, i%256)
		seen[shardFor(shards, ip)] = true
	}
	if len(seen) < rateLimitShardCount/2 {
		t.Errorf("1000 distinct IPs landed on only %d of %d shards, want a reasonable spread (at least half)", len(seen), rateLimitShardCount)
	}
}
