package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type Redis struct {
	Client *redis.Client
}

func NewRedis(client *redis.Client) *Redis {
	return &Redis{Client: client}
}

const lockTTL = 15 * time.Minute

// Lock a single seat
func (r *Redis) AddOTP(otp, orderID string) (bool, error) {
	key := "OTP_lock:" + orderID
	ok, err := r.Client.SetNX(context.Background(), key, otp, lockTTL).Result()
	return ok, err
}

// Unlock a single seat
func (r *Redis) RemoveOTP(orderID string) error {
	ctx := context.Background()
	key := fmt.Sprintf("OTP_lock:%s", orderID)
	val, err := r.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil // already unlocked
	}
	if err != nil {
		return err
	}
	if val == orderID {
		_, err := r.Client.Del(ctx, key).Result()
		return err
	}
	return nil // do not unlock if not owned by this order
}
