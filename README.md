# VM DHCP Controller

## Features

- DHCP service for virtual machines
  - IP pool declaration
  - Per-VM network configuration
  - Static lease support (pre-defined MAC/IP addresses mapping)
- Semi-stateless design and resiliency built in the heart
  - States are always kept in etcd
  - Able to reconstruct DHCP leases even when the agent is destroyed and restarted
- Harvester integration
  - Network topology-aware agent (DHCP server) deployment
  - Auto-create IP pool along with **VM Network** creation
  - Auto-create network configuration during VM creation

## Architecture

Introduced CRDs:

- IPPool
- VirtualNetworkNetworkConfig

Components:

- Manager (control plane)
  - Manage the lifecycle of the agent for each IPPool
  - [WIP] Create/remove IPPool when **VM Network** is created/deleted
  - [WIP] Create/remove VirtualMachineNetworkConfig when VM is created/deleted
- Agent (data plane)
  - Maintain the IPAM and DHCP leases for the IP pool it manages
  - Handle DHCP requests

## Develop

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

## Usage

Create **VM Network** `net-48` before proceeding

Create IPPool object:

```
$ cat <<EOF | kubectl apply -f -
apiVersion: network.harvesterhci.io/v1alpha1
kind: IPPool
metadata:
  name: ippool-1
  namespace: default
  labels:
    k8s.cni.cncf.io/net-attach-def: net-48
spec:
  ipv4Config:
    serverIP: 192.168.48.77
    cidr: 192.168.48.0/24
    pool:
      start: 192.168.48.81
      end: 192.168.48.90
      exclude:
      - 192.168.48.81
      - 192.168.48.90
    router: 192.168.48.1
    dns:
    - 1.1.1.1
    domainName: aibao.moe
    domainSearch:
    - aibao.moe
    ntp:
    - pool.ntp.org
    leaseTime: 300
  networkName: net-48
EOF
```

Create VirtualMachineNetworkConfig object:

```
$ cat <<EOF | kubectl apply -f -
apiVersion: network.harvesterhci.io/v1alpha1
kind: VirtualMachineNetworkConfig
metadata:
  name: test-vm
  namespace: default 
  labels:
    harvesterhci.io/vmName: test-vm
spec:
  vmName: test-vm
  networkConfig:
  - macAddress: fa:cf:8e:50:82:fc
    networkName: net-48
EOF
```