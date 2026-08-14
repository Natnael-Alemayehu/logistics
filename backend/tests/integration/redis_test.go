//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	redismodule "github.com/natnael-alemayehu/logistics/pkg/redis"
	rediscontainer "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestRedisIntegration(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := rediscontainer.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	defer redisContainer.Terminate(ctx)

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis host: %v", err)
	}
	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get redis port: %v", err)
	}

	client, err := redismodule.NewClient(redismodule.Config{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer client.Close()

	t.Run("SetAndGet", func(t *testing.T) {
		ctx := context.Background()
		key := "test:setandget"
		err := client.Set(ctx, key, "test-value", time.Minute)
		if err != nil {
			t.Fatalf("failed to set: %v", err)
		}

		val, err := client.Get(ctx, key)
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}
		if val != "test-value" {
			t.Fatalf("expected test-value, got %s", val)
		}
	})

	t.Run("GetNonExistentKey", func(t *testing.T) {
		ctx := context.Background()
		_, err := client.Get(ctx, "nonexistent:key")
		if err == nil {
			t.Fatal("expected error for non-existent key")
		}
	})

	t.Run("SetWithExpiration", func(t *testing.T) {
		ctx := context.Background()
		key := "test:expiring"
		err := client.Set(ctx, key, "expiring-value", 2*time.Second)
		if err != nil {
			t.Fatalf("failed to set: %v", err)
		}

		val, err := client.Get(ctx, key)
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}
		if val != "expiring-value" {
			t.Fatalf("expected expiring-value, got %s", val)
		}

		time.Sleep(3 * time.Second)

		_, err = client.Get(ctx, key)
		if err == nil {
			t.Fatal("expected error for expired key")
		}
	})

	t.Run("Incr", func(t *testing.T) {
		ctx := context.Background()
		key := "test:incr"

		val, err := client.Incr(ctx, key)
		if err != nil {
			t.Fatalf("failed to incr: %v", err)
		}
		if val != 1 {
			t.Fatalf("expected 1, got %d", val)
		}

		val, err = client.Incr(ctx, key)
		if err != nil {
			t.Fatalf("failed to incr: %v", err)
		}
		if val != 2 {
			t.Fatalf("expected 2, got %d", val)
		}

		val, err = client.Incr(ctx, key)
		if err != nil {
			t.Fatalf("failed to incr: %v", err)
		}
		if val != 3 {
			t.Fatalf("expected 3, got %d", val)
		}
	})

	t.Run("TTL", func(t *testing.T) {
		ctx := context.Background()
		key := "test:ttl"

		err := client.Set(ctx, key, "ttl-value", 30*time.Second)
		if err != nil {
			t.Fatalf("failed to set: %v", err)
		}

		ttl, err := client.TTL(ctx, key)
		if err != nil {
			t.Fatalf("failed to get TTL: %v", err)
		}
		if ttl <= 0 || ttl > 30*time.Second {
			t.Fatalf("expected TTL between 0-30s, got %v", ttl)
		}
	})

	t.Run("Expire", func(t *testing.T) {
		ctx := context.Background()
		key := "test:expire"

		err := client.Set(ctx, key, "expire-value", 0)
		if err != nil {
			t.Fatalf("failed to set: %v", err)
		}

		ttl, err := client.TTL(ctx, key)
		if err != nil {
			t.Fatalf("failed to get TTL: %v", err)
		}
		if ttl != -1 {
			t.Fatalf("expected TTL -1 (no expiration), got %v", ttl)
		}

		err = client.Expire(ctx, key, 10*time.Second)
		if err != nil {
			t.Fatalf("failed to set expiration: %v", err)
		}

		ttl, err = client.TTL(ctx, key)
		if err != nil {
			t.Fatalf("failed to get TTL: %v", err)
		}
		if ttl <= 0 || ttl > 10*time.Second {
			t.Fatalf("expected TTL between 0-10s, got %v", ttl)
		}
	})

	t.Run("SetOperations", func(t *testing.T) {
		ctx := context.Background()
		key := "test:set"

		err := client.SAdd(ctx, key, "member1", "member2", "member3")
		if err != nil {
			t.Fatalf("failed to sadd: %v", err)
		}

		members, err := client.SMembers(ctx, key)
		if err != nil {
			t.Fatalf("failed to smembers: %v", err)
		}
		if len(members) != 3 {
			t.Fatalf("expected 3 members, got %d", len(members))
		}

		err = client.SRem(ctx, key, "member2")
		if err != nil {
			t.Fatalf("failed to srem: %v", err)
		}

		members, err = client.SMembers(ctx, key)
		if err != nil {
			t.Fatalf("failed to smembers: %v", err)
		}
		if len(members) != 2 {
			t.Fatalf("expected 2 members after removal, got %d", len(members))
		}
	})

	t.Run("Del", func(t *testing.T) {
		ctx := context.Background()
		key := "test:del"

		err := client.Set(ctx, key, "del-value", time.Minute)
		if err != nil {
			t.Fatalf("failed to set: %v", err)
		}

		_, err = client.Get(ctx, key)
		if err != nil {
			t.Fatalf("failed to get: %v", err)
		}

		err = client.Del(ctx, key)
		if err != nil {
			t.Fatalf("failed to del: %v", err)
		}

		_, err = client.Get(ctx, key)
		if err == nil {
			t.Fatal("expected error for deleted key")
		}
	})

	t.Run("Exists", func(t *testing.T) {
		ctx := context.Background()
		key := "test:exists"

		count, err := client.Exists(ctx, key)
		if err != nil {
			t.Fatalf("failed to check exists: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected 0 for non-existent key, got %d", count)
		}

		err = client.Set(ctx, key, "exists-value", time.Minute)
		if err != nil {
			t.Fatalf("failed to set: %v", err)
		}

		count, err = client.Exists(ctx, key)
		if err != nil {
			t.Fatalf("failed to check exists: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 for existing key, got %d", count)
		}
	})

	t.Run("Health", func(t *testing.T) {
		ctx := context.Background()
		err := client.Health(ctx)
		if err != nil {
			t.Fatalf("health check failed: %v", err)
		}
	})
}

func TestRedisIntegration_MultipleKeys(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := rediscontainer.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	defer redisContainer.Terminate(ctx)

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis host: %v", err)
	}
	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get redis port: %v", err)
	}

	client, err := redismodule.NewClient(redismodule.Config{
		Addr: fmt.Sprintf("%s:%s", host, port.Port()),
	})
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	defer client.Close()

	t.Run("DeleteMultipleKeys", func(t *testing.T) {
		ctx := context.Background()

		client.Set(ctx, "multi:1", "value1", time.Minute)
		client.Set(ctx, "multi:2", "value2", time.Minute)
		client.Set(ctx, "multi:3", "value3", time.Minute)

		count, err := client.Exists(ctx, "multi:1", "multi:2", "multi:3")
		if err != nil {
			t.Fatalf("failed to check exists: %v", err)
		}
		if count != 3 {
			t.Fatalf("expected 3 keys to exist, got %d", count)
		}

		err = client.Del(ctx, "multi:1", "multi:2", "multi:3")
		if err != nil {
			t.Fatalf("failed to delete multiple keys: %v", err)
		}

		count, err = client.Exists(ctx, "multi:1", "multi:2", "multi:3")
		if err != nil {
			t.Fatalf("failed to check exists: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected 0 keys to exist after deletion, got %d", count)
		}
	})
}

func TestRedisIntegration_ConnectionFailure(t *testing.T) {
	_, err := redismodule.NewClient(redismodule.Config{
		Addr: "localhost:9999",
	})
	if err == nil {
		t.Fatal("expected error for invalid connection")
	}
}
