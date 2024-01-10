package cache

import (
	"fmt"
	"sync"
)

type MACSet struct {
	macs map[string]struct{}
}

type CacheAllocator struct {
	cache map[string]MACSet
	mutex sync.RWMutex
}

func New() *CacheAllocator {
	return NewCacheAllocator()
}

func NewCacheAllocator() *CacheAllocator {
	return &CacheAllocator{
		cache: make(map[string]MACSet),
	}
}

func (c *CacheAllocator) NewMACIPMap(name string) error {
	c.cache[name] = MACSet{
		macs: make(map[string]struct{}),
	}
	return nil
}

func (c *CacheAllocator) AddEntry(name, macAddress string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Sanity check
	if _, exists := c.cache[name]; !exists {
		return fmt.Errorf("network %s does not exist", name)
	}

	c.cache[name].macs[macAddress] = struct{}{}

	return nil
}

func (c *CacheAllocator) DeleteEntry(name, macAddress string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Sanity check
	if _, exists := c.cache[name]; !exists {
		return fmt.Errorf("network %s does not exist", name)
	}

	delete(c.cache[name].macs, macAddress)

	return nil
}

func (c *CacheAllocator) HasEntry(name, macAddress string) (bool, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Sanity check
	if _, exists := c.cache[name]; !exists {
		return false, fmt.Errorf("network %s does not exist", name)
	}

	_, exists := c.cache[name].macs[macAddress]

	return exists, nil
}
