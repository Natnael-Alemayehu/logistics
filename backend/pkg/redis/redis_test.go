package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func setupTestRedis(t *testing.T) (*Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client, err := NewClient(Config{
		Addr: mr.Addr(),
	})
	if err != nil {
		t.Fatalf("failed to create Redis client: %v", err)
	}

	return client, mr
}

func TestNewClient(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	client, err := NewClient(Config{
		Addr: mr.Addr(),
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatal("expected client, got nil")
	}
}

func TestNewClientWithDefaults(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	client, err := NewClient(Config{
		Addr: mr.Addr(),
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer client.Close()

	ctx := context.Background()
	if err := client.Health(ctx); err != nil {
		t.Fatalf("expected healthy connection, got: %v", err)
	}
}

func TestSetGet(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	ctx := context.Background()

	err := client.Set(ctx, "test-key", "test-value", 0)
	if err != nil {
		t.Fatalf("expected no error on Set, got: %v", err)
	}

	value, err := client.Get(ctx, "test-key")
	if err != nil {
		t.Fatalf("expected no error on Get, got: %v", err)
	}

	if value != "test-value" {
		t.Fatalf("expected 'test-value', got '%s'", value)
	}
}

func TestSetWithTTL(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	ctx := context.Background()

	err := client.Set(ctx, "ttl-key", "ttl-value", 5*time.Second)
	if err != nil {
		t.Fatalf("expected no error on Set, got: %v", err)
	}

	ttl, err := client.TTL(ctx, "ttl-key")
	if err != nil {
		t.Fatalf("expected no error on TTL, got: %v", err)
	}

	if ttl <= 0 || ttl > 5*time.Second {
		t.Fatalf("expected TTL between 0 and 5s, got %v", ttl)
	}
}

func TestIncr(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	ctx := context.Background()

	val, err := client.Incr(ctx, "counter")
	if err != nil {
		t.Fatalf("expected no error on Incr, got: %v", err)
	}

	if val != 1 {
		t.Fatalf("expected 1, got %d", val)
	}

	val, err = client.Incr(ctx, "counter")
	if err != nil {
		t.Fatalf("expected no error on second Incr, got: %v", err)
	}

	if val != 2 {
		t.Fatalf("expected 2, got %d", val)
	}
}

func TestDel(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	ctx := context.Background()

	client.Set(ctx, "del-key", "del-value", 0)

	err := client.Del(ctx, "del-key")
	if err != nil {
		t.Fatalf("expected no error on Del, got: %v", err)
	}

	_, err = client.Get(ctx, "del-key")
	if err == nil {
		t.Fatal("expected error on Get after Del, got nil")
	}
}

func TestExists(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	ctx := context.Background()

	count, err := client.Exists(ctx, "exists-key")
	if err != nil {
		t.Fatalf("expected no error on Exists, got: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}

	client.Set(ctx, "exists-key", "value", 0)

	count, err = client.Exists(ctx, "exists-key")
	if err != nil {
		t.Fatalf("expected no error on Exists, got: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
}

func TestSetOperations(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	ctx := context.Background()

	err := client.SAdd(ctx, "set-key", "member1", "member2", "member3")
	if err != nil {
		t.Fatalf("expected no error on SAdd, got: %v", err)
	}

	members, err := client.SMembers(ctx, "set-key")
	if err != nil {
		t.Fatalf("expected no error on SMembers, got: %v", err)
	}

	if len(members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(members))
	}

	err = client.SRem(ctx, "set-key", "member1")
	if err != nil {
		t.Fatalf("expected no error on SRem, got: %v", err)
	}

	members, err = client.SMembers(ctx, "set-key")
	if err != nil {
		t.Fatalf("expected no error on SMembers, got: %v", err)
	}

	if len(members) != 2 {
		t.Fatalf("expected 2 members after SRem, got %d", len(members))
	}
}

func TestExpire(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	ctx := context.Background()

	client.Set(ctx, "expire-key", "value", 0)

	err := client.Expire(ctx, "expire-key", 10*time.Second)
	if err != nil {
		t.Fatalf("expected no error on Expire, got: %v", err)
	}

	ttl, err := client.TTL(ctx, "expire-key")
	if err != nil {
		t.Fatalf("expected no error on TTL, got: %v", err)
	}

	if ttl <= 0 || ttl > 10*time.Second {
		t.Fatalf("expected TTL between 0 and 10s, got %v", ttl)
	}
}

func TestHealth(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	ctx := context.Background()

	if err := client.Health(ctx); err != nil {
		t.Fatalf("expected healthy connection, got: %v", err)
	}
}

func TestClose(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	client, err := NewClient(Config{
		Addr: mr.Addr(),
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if err := client.Close(); err != nil {
		t.Fatalf("expected no error on Close, got: %v", err)
	}
}

func TestRaw(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()
	defer mr.Close()

	raw := client.Raw()
	if raw == nil {
		t.Fatal("expected raw client, got nil")
	}
}
