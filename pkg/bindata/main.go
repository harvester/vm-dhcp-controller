package main

import (
	"github.com/go-bindata/go-bindata/v3"
)

const CRDPath = "./chart/crds"

func main() {
	c := &bindata.Config{
		Input: []bindata.InputConfig{
			{
				Path: CRDPath,
			},
		},
		Output:  "./pkg/data/data.go",
		Package: "data",
	}

	err := bindata.Translate(c)
	if err != nil {
		panic(err)
	}
}
