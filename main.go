//go:generate go run pkg/codegen/cleanup/main.go
//go:generate go run pkg/codegen/main.go

package main

import (
	"github.com/starbops/vm-dhcp-controller/cmd"
)

var (
	AppVersion = "dev"
	GitCommit  = "commit"
)

func main() {
	cmd.Execute()
}
