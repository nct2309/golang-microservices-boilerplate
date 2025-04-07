package cache

import (
	"sync"
)

// Cache is a simple in-memory cache
type Cache struct {
	mu    sync.RWMutex
	store map[string]interface{}
}

// NewCache creates a new Cache instance
func NewCache() *Cache {
	return &Cache{
		store: make(map[string]interface{}),
	}
}

// Get retrieves an item from the cache
func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.store[key]
	return value, exists
}

// Set adds an item to the cache
func (c *Cache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}

// Delete removes an item from the cache
func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
}

// Clear removes all items from the cache
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store = make(map[string]interface{})
}