package redisnotify

import (
    "context"
    "testing"

    "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
)

func TestPublisher_Publish(t *testing.T) {
    mr, err := miniredis.Run()
    if err != nil {
        t.Fatalf("miniredis.Run() error = %v", err)
    }
    defer mr.Close()

    client := redis.NewClient(&redis.Options{
        Addr: mr.Addr(),
    })

    p := NewPublisher(client, "kd48:lobby:config:notify")
    err = p.Publish(context.Background(), "TestConfig", 1)
    if err != nil {
        t.Fatalf("Publish() error = %v", err)
    }
}
