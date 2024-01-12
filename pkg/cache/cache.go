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

func (a *CacheAllocator) NewMACSet(name string) error {
	a.cache[name] = MACSet{
		macs: make(map[string]net.IP),
	}
	return nil
}

func (a *CacheAllocator) DeleteMACSet(name string) {
	delete(a.cache, name)
}

func (a *CacheAllocator) AddMAC(name, macAddress, ipAddress string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Sanity check
	if _, exists := a.cache[name]; !exists {
		return fmt.Errorf("network %s does not exist", name)
	}

	a.cache[name].macs[macAddress] = net.ParseIP(ipAddress)

	return nil
}

func (a *CacheAllocator) DeleteMAC(name, macAddress string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Sanity check
	if _, exists := a.cache[name]; !exists {
		return fmt.Errorf("network %s does not exist", name)
	}

	delete(a.cache[name].macs, macAddress)

	return nil
}

func (a *CacheAllocator) HasMAC(name, macAddress string) (bool, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// Sanity check
	if _, exists := a.cache[name]; !exists {
		return false, fmt.Errorf("network %s does not exist", name)
	}

	_, exists := a.cache[name].macs[macAddress]

	return exists, nil
}

func (a *CacheAllocator) GetIPByMAC(name, macAddress string) (string, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// Sanity check
	if _, exists := a.cache[name]; !exists {
		return "", fmt.Errorf("network %s does not exist", name)
	}

	ipAddress, exists := a.cache[name].macs[macAddress]
	if !exists {
		return "", fmt.Errorf("mac %s not found in network %s", macAddress, name)
	}

	return ipAddress.String(), nil
}

func (a *CacheAllocator) ListAll(name string) (map[string]string, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// Sanity check
	if _, exists := a.cache[name]; !exists {
		return nil, fmt.Errorf("network %s does not exist", name)
	}

	macs := make(map[string]string, len(a.cache[name].macs))
	for mac, ip := range a.cache[name].macs {
		macs[mac] = ip.String()
	}

	return macs, nil
}
