package cache

import (
	"fmt"
	"net"
	"sync"
)

type MACSet struct {
	macs map[string]net.IP
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

func (c *CacheAllocator) NewMACSet(name string) error {
	c.cache[name] = MACSet{
		macs: make(map[string]net.IP),
	}
	return nil
}

func (c *CacheAllocator) DeleteMACSet(name string) {
	delete(c.cache, name)
}

func (c *CacheAllocator) AddMAC(name, macAddress, ipAddress string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Sanity check
	if _, exists := c.cache[name]; !exists {
		return fmt.Errorf("network %s does not exist", name)
	}

	c.cache[name].macs[macAddress] = net.ParseIP(ipAddress)

	return nil
}

func (c *CacheAllocator) DeleteMAC(name, macAddress string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Sanity check
	if _, exists := c.cache[name]; !exists {
		return fmt.Errorf("network %s does not exist", name)
	}

	delete(c.cache[name].macs, macAddress)

	return nil
}

func (c *CacheAllocator) HasMAC(name, macAddress string) (bool, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Sanity check
	if _, exists := c.cache[name]; !exists {
		return false, fmt.Errorf("network %s does not exist", name)
	}

	_, exists := c.cache[name].macs[macAddress]

	return exists, nil
}

func (c *CacheAllocator) GetIPByMAC(name, macAddress string) (string, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Sanity check
	if _, exists := c.cache[name]; !exists {
		return "", fmt.Errorf("network %s does not exist", name)
	}

	ipAddress, exists := c.cache[name].macs[macAddress]
	if !exists {
		return "", fmt.Errorf("mac %s not found in network %s", macAddress, name)
	}

	return ipAddress.String(), nil
}
