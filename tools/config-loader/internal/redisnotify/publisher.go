package redisnotify

import (
    "context"
    "encoding/json"

    "github.com/redis/go-redis/v9"
)

type Publisher struct {
    client  *redis.Client
    channel string
}

func NewPublisher(client *redis.Client, channel string) *Publisher {
    return &Publisher{
        client:  client,
        channel: channel,
    }
}

func (p *Publisher) Publish(ctx context.Context, configName string, revision int64) error {
    msg := map[string]interface{}{
        "kind":        "lobby_config_published",
        "config_name": configName,
        "revision":    revision,
    }

    bytes, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    return p.client.Publish(ctx, p.channel, string(bytes)).Err()
}
