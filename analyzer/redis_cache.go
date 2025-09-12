package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// RedisCache implements VulnerabilityCache using Redis
type RedisCache struct {
	// In production, this would use github.com/go-redis/redis client
	// For now, we'll use an in-memory map for simplicity
	data map[string][]byte
	ttls map[string]time.Time
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(redisURL string) *RedisCache {
	// In production:
	//     Addr: redisURL,
	// })

	return &RedisCache{
		data: make(map[string][]byte),
		ttls: make(map[string]time.Time),
	}
}

// Get retrieves a vulnerability from cache
func (rc *RedisCache) Get(key string) (*VulnerabilityInfo, error) {
	// Check if key exists and not expired
	expiry, exists := rc.ttls[key]
	if !exists {
		return nil, fmt.Errorf("key not found")
	}

	if !time.Now().Before(expiry) {
		// Expired, clean up
		delete(rc.data, key)
		delete(rc.ttls, key)
		return nil, fmt.Errorf("key not found")
	}

	data, ok := rc.data[key]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}

	var info VulnerabilityInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// Set stores a vulnerability in cache with TTL
func (rc *RedisCache) Set(key string, value *VulnerabilityInfo, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	rc.data[key] = data
	rc.ttls[key] = time.Now().Add(ttl)

	// In production with Redis:

	return nil
}

// GetLastUpdate gets the last update timestamp
func (rc *RedisCache) GetLastUpdate() (time.Time, error) {
	if data, ok := rc.data["last_update"]; ok {
		var timestamp time.Time
		if err := json.Unmarshal(data, &timestamp); err != nil {
			return time.Time{}, err
		}
		return timestamp, nil
	}
	return time.Time{}, fmt.Errorf("no last update found")
}

// SetLastUpdate sets the last update timestamp
func (rc *RedisCache) SetLastUpdate(timestamp time.Time) error {
	data, err := json.Marshal(timestamp)
	if err != nil {
		return err
	}
	rc.data["last_update"] = data
	return nil
}

// BatchGet retrieves multiple vulnerabilities at once
func (rc *RedisCache) BatchGet(keys []string) (map[string]*VulnerabilityInfo, error) {
	results := make(map[string]*VulnerabilityInfo)

	for _, key := range keys {
		if info, err := rc.Get(key); err == nil {
			results[key] = info
		}
	}

	return results, nil
}

// Clear clears all cached data
func (rc *RedisCache) Clear() error {
	rc.data = make(map[string][]byte)
	rc.ttls = make(map[string]time.Time)
	return nil
}

// GetStats returns cache statistics
func (rc *RedisCache) GetStats() map[string]interface{} {
	activeKeys := 0
	expiredKeys := 0
	now := time.Now()

	for key, expiry := range rc.ttls {
		if now.Before(expiry) {
			activeKeys++
		} else {
			expiredKeys++
			// Clean up expired keys
			delete(rc.data, key)
			delete(rc.ttls, key)
		}
	}

	return map[string]interface{}{
		"total_keys":   len(rc.data),
		"active_keys":  activeKeys,
		"expired_keys": expiredKeys,
	}
}

// StartCleanup starts a background goroutine to clean up expired entries
func (rc *RedisCache) StartCleanup(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rc.cleanup()
			}
		}
	}()
}

func (rc *RedisCache) cleanup() {
	now := time.Now()
	for key, expiry := range rc.ttls {
		if now.After(expiry) {
			delete(rc.data, key)
			delete(rc.ttls, key)
		}
	}
}
