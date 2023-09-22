package cache

import (
	"fmt"

	kihv1 "github.com/joeyloman/kubevirt-ip-helper/pkg/apis/kubevirtiphelper.k8s.binbash.org/v1"

	log "github.com/sirupsen/logrus"
)

type CacheAllocator struct {
	ipPoolCache map[string]kihv1.IPPool
}

func NewCacheAllocator() *CacheAllocator {
	ipPoolCache := make(map[string]kihv1.IPPool)

	return &CacheAllocator{
		ipPoolCache: ipPoolCache,
	}
}

func New() *CacheAllocator {
	return NewCacheAllocator()
}

func (c *CacheAllocator) Add(t interface{}) (err error) {
	switch t.(type) {
	case *kihv1.IPPool:
		// TODO: remove?
		log.Debugf("(cache.AddToCache) adding pool for %s", t.(*kihv1.IPPool).Spec.NetworkName)

		if _, exists := c.ipPoolCache[t.(*kihv1.IPPool).Spec.NetworkName]; exists {
			return fmt.Errorf("IPPool %s already exists in cache", t.(*kihv1.IPPool).Spec.NetworkName)
		}

		c.ipPoolCache[t.(*kihv1.IPPool).Spec.NetworkName] = *t.(*kihv1.IPPool)
	}

	return
}

func (c *CacheAllocator) Check(t interface{}) bool {
	switch t.(type) {
	case *kihv1.IPPool:
		_, exists := c.ipPoolCache[t.(*kihv1.IPPool).Spec.NetworkName]
		return exists
	}

	return false
}

func (c *CacheAllocator) Get(t string, name string) (i interface{}, err error) {
	switch t {
	case "pool":
		// TODO: remove
		log.Debugf("(cache.Get) returning pool for %s", name)

		if _, exists := c.ipPoolCache[name]; !exists {
			return i, fmt.Errorf("IPPool %s does not exists in cache", name)
		}

		return c.ipPoolCache[name], nil
	}
	return
}

func (c *CacheAllocator) Delete(t string, name string) (err error) {
	switch t {
	case "pool":
		// TODO: remove
		log.Debugf("(cache.DeleteFromCache) deleting pool for %s", name)

		if _, exists := c.ipPoolCache[name]; !exists {
			return fmt.Errorf("IPPool %s does not exists in cache", name)
		}

		delete(c.ipPoolCache, name)
	}

	return
}

func (c *CacheAllocator) Usage(t string) {
	switch t {
	case "pool":
		for subnet, pool := range c.ipPoolCache {
			log.Infof("(cache.Usage) ipPoolCache: key=%s, subnet=%s, network=%s, serverip=%s",
				subnet, pool.Spec.IPv4Config.Subnet, pool.Spec.NetworkName, pool.Spec.IPv4Config.ServerIP)
		}
	}
}
