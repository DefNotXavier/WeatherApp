// Package auth keeps admin login sessions in Redis.
//
// The PHP version used PHP's file-based sessions. Keeping them in Redis means
// the Go server doesn't hold any state itself, so you could run more than one
// copy of it behind a load balancer.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const SessionTTL = 8 * time.Hour

type RedisSessions struct {
	RDB *redis.Client
}

func sessionKey(token string) string { return "session:" + token }

// Create starts a session and returns its random token for the cookie.
func (s RedisSessions) Create(ctx context.Context, username string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	return token, s.RDB.Set(ctx, sessionKey(token), username, SessionTTL).Err()
}

// Lookup returns the username for a session token, if it hasn't expired.
func (s RedisSessions) Lookup(ctx context.Context, token string) (string, bool, error) {
	if token == "" {
		return "", false, nil
	}
	u, err := s.RDB.Get(ctx, sessionKey(token)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return u, true, nil
}

func (s RedisSessions) Delete(ctx context.Context, token string) error {
	return s.RDB.Del(ctx, sessionKey(token)).Err()
}

func (s RedisSessions) Ping(ctx context.Context) error {
	return s.RDB.Ping(ctx).Err()
}
