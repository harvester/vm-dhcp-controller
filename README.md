# VM DHCP Controller

## Architecture

Components:

- vm-dhcp-manager
- vm-dhcp-agent

Introduced CRDs:

- IPPool
- VirtualNetworkNetworkConfig

## Development

To generate the CRDs/controllers/clientsets:

```
go generate
```

## Build

To build the VM DHCP controller and package it into a container image:

```
make
```

If you're on an Apple Silicon Mac with Docker Desktop or OrbStack, and want to build the binary and the image that could be run on a Linux x86_64 box, you'd like to do the following:

```
export ARCH=amd64
make
```

## Run

To run the manager locally attaching to a remote cluster:

```
# Make sure you have the correct config and context set
export KUBECONFIG="$HOME/cluster.yaml"

make run ARGS="manager"
```

Same for the agent (for testing purposes):

```
make run ARGS="agent"
```

## Install

To install the VM DHCP controller using Helm:

```
helm upgrade --install harvester-vm-dhcp-controller ./chart --namespace=harvester-system --create-namespace
```

The agents will be scaffolded dynamically according to the requests.
