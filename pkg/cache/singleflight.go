package cache

import "sync"

// SingleFlight ensures that only one goroutine executes a given function for a
// given key at a time. Other goroutines calling Do with the same key wait for
// the first to complete and receive the same result.
//
// This prevents "cache stampede" — when a hot cache entry expires and dozens
// of concurrent requests all try to backfill from DB simultaneously.
type SingleFlight struct {
	mu    sync.Mutex
	calls map[string]*call
}

type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// NewSingleFlight creates a new SingleFlight instance.
func NewSingleFlight() *SingleFlight {
	return &SingleFlight{
		calls: make(map[string]*call),
	}
}

// Do executes fn for key. If another goroutine is already executing fn for
// this key, Do blocks until that call returns, then returns the same result.
// The caller must type-assert the returned value.
func (s *SingleFlight) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	s.mu.Lock()
	if c, ok := s.calls[key]; ok {
		s.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := &call{}
	c.wg.Add(1)
	s.calls[key] = c
	s.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	s.mu.Lock()
	delete(s.calls, key)
	s.mu.Unlock()

	return c.val, c.err
}

// Delete removes any in-flight call for key (e.g., after the lock scope ends
// and we want to allow a fresh call next time).
func (s *SingleFlight) Delete(key string) {
	s.mu.Lock()
	delete(s.calls, key)
	s.mu.Unlock()
}
