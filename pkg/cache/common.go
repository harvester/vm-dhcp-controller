package cache

type CacheAllocatorBuilder struct {
	cacheAllocator *CacheAllocator
}

func NewCacheAllocatorBuilder() *CacheAllocatorBuilder {
	return &CacheAllocatorBuilder{
		cacheAllocator: New(),
	}
}

func (b *CacheAllocatorBuilder) MACSet(name string) *CacheAllocatorBuilder {
	_ = b.cacheAllocator.NewMACSet(name)
	return b
}

func (b *CacheAllocatorBuilder) Add(name, macAddress, ipAddress string) *CacheAllocatorBuilder {
	_ = b.cacheAllocator.AddMAC(name, macAddress, ipAddress)
	return b
}

func (b *CacheAllocatorBuilder) Build() *CacheAllocator {
	return b.cacheAllocator
}
