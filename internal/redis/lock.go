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

const lockTTL = 5 * time.Minute

// Lock a single seat
func (r *Redis) AddOTP(otp string, orderID string) (bool, error) {
	key := "OTP_lock:" + orderID
	ok, err := r.Client.SetNX(context.Background(), key, otp, lockTTL).Result()
	if err != nil {
		fmt.Printf("AddOTP: failed to set lock for orderID=%s, err=%v\n", orderID, err)
	} else {
		fmt.Printf("AddOTP: lock set for orderID=%s, success=%v\n", orderID, ok)
	}
	return ok, err
}

// Unlock a single seat
func (r *Redis) RemoveOTP(orderID string) error {
	ctx := context.Background()
	key := fmt.Sprintf("OTP_lock:%s", orderID)
	val, err := r.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		fmt.Printf("RemoveOTP: lock already removed for orderID=%s\n", orderID)
		return nil // already unlocked
	}
	if err != nil {
		fmt.Printf("RemoveOTP: error getting lock for orderID=%s, err=%v\n", orderID, err)
		return err
	}
	if val == orderID {
		_, err := r.Client.Del(ctx, key).Result()
		if err != nil {
			fmt.Printf("RemoveOTP: failed to remove lock for orderID=%s, err=%v\n", orderID, err)
		} else {
			fmt.Printf("RemoveOTP: lock removed for orderID=%s\n", orderID)
		}
		return err
	}
	fmt.Printf("RemoveOTP: lock not owned by orderID=%s, current value=%s\n", orderID, val)
	return nil // do not unlock if not owned by this order
}

func (r *Redis) IsOTPLocked(orderID string) (bool, error) {
	key := "OTP_lock:" + orderID
	ctx := context.Background()

	val, err := r.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		fmt.Printf("IsOTPLocked: lock not found for orderID=%s\n", orderID)
		return false, nil // Unlocked
	}
	if err != nil {
		fmt.Printf("IsOTPLocked: error getting lock for orderID=%s, err=%v\n", orderID, err)
		return false, err
	}
	fmt.Printf("IsOTPLocked: lock found for orderID=%s, value=%s\n", orderID, val)
	return true, nil // Locked, return OTP value
}

func (r *Redis) GetOTP(orderID string) (string, error) {
	key := "OTP_lock:" + orderID
	ctx := context.Background()

	val, err := r.Client.Get(ctx, key).Result()
	if err == redis.Nil {
		fmt.Printf("GetOTP: lock not found for orderID=%s\n", orderID)
		return "", nil // Unlocked
	}
	if err != nil {
		fmt.Printf("GetOTP: error getting lock for orderID=%s, err=%v\n", orderID, err)
		return "", err
	}
	fmt.Printf("GetOTP: lock found for orderID=%s, value=%s\n", orderID, val)
	return val, nil // Return OTP value
}
