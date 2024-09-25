package counters

import (
	"counter-service/cache"
	"counter-service/storage"
	"time"
)

type CounterService struct {
	cache   *cache.RedisCache
	storage *storage.DB
}

func NewCounterService(cache *cache.RedisCache, storage *storage.DB) *CounterService {
	return &CounterService{
		cache:   cache,
		storage: storage,
	}
}

func (s *CounterService) GetUnreadCount(userID string) (int64, error) {
	// Try to get counter from Redis
	count, err := s.cache.Get(userID)
	if err == nil {
		return count, nil
	}

	// If not in Redis, fall back to DB
	count, err = s.storage.GetCounter(userID)
	if err != nil {
		return 0, err
	}

	// Cache the counter value in Redis
	_ = s.cache.Set(userID, count, 5*time.Minute)
	return count, nil
}

func (s *CounterService) IncrementUnreadCount(userID string, increment int64) error {
	// Increment in Redis
	err := s.cache.IncrBy(userID, increment)
	if err != nil {
		return err
	}

	// Get updated counter value and persist in the DB
	newCount, err := s.cache.Get(userID)
	if err != nil {
		return err
	}

	// Persist new value to DB
	errUpdate := s.storage.UpdateCounter(userID, newCount)
	if errUpdate != nil {
		errDecr := s.cache.DecrBy(userID, increment)
		if errDecr != nil {
			return errDecr
		}
		return errUpdate
	}
	return err
}
