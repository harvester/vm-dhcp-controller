# VM DHCP Controller

Components:

- vm-dhcp-manager
- vm-dhcp-agent

Newly introduced CRDs:

- IPPool
- VirtualNetworkNetworkConfig

```
go run pkg/codegen/cleanup/main.go
go run pkg/codegen/main.go
```

```
make
```