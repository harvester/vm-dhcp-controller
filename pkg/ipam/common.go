package ipam

type IPAllocatorBuilder struct {
	ipAllocator *IPAllocator
}

func NewIPAllocatorBuilder() *IPAllocatorBuilder {
	return &IPAllocatorBuilder{
		ipAllocator: New(),
	}
}

func (b *IPAllocatorBuilder) IPSubnet(name, cidr, start, end string) *IPAllocatorBuilder {
	_ = b.ipAllocator.NewIPSubnet(name, cidr, start, end)
	return b
}

func (b *IPAllocatorBuilder) Revoke(name string, ipAddressList ...string) *IPAllocatorBuilder {
	for _, ip := range ipAddressList {
		_ = b.ipAllocator.RevokeIP(name, ip)
	}
	return b
}

func (b *IPAllocatorBuilder) Allocate(name string, ipAddressList ...string) *IPAllocatorBuilder {
	for _, ip := range ipAddressList {
		_, _ = b.ipAllocator.AllocateIP(name, ip)
	}
	return b
}

func (b *IPAllocatorBuilder) Build() *IPAllocator {
	return b.ipAllocator
}
