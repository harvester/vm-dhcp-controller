// Code generated by go-bindata. (@generated) DO NOT EDIT.

 //Package data generated by go-bindata.// sources:
// chart/crds/network.harvesterhci.io_ippools.yaml
// chart/crds/network.harvesterhci.io_virtualmachinenetworkconfigs.yaml
package data

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// ModTime return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _chartCrdsNetworkHarvesterhciIo_ippoolsYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xcc\x19\x5d\x6f\xe3\xc6\xf1\x5d\xbf\x62\x8a\x3e\x38\x01\x4c\x19\x87\x14\x45\x21\xe0\xd0\x3a\xb2\x9a\x08\x71\x1d\x43\xb2\xaf\x08\x8a\x3e\x8c\xb8\x23\x69\xe3\xe5\x2e\xb3\x1f\xb2\xdd\x5c\xfe\x7b\x31\x4b\xd2\xa2\x28\x92\xa6\x75\x77\x41\xf6\x49\x9a\xdd\x9d\xef\x99\x9d\x19\x26\x49\x32\xc2\x5c\x7e\x20\xeb\xa4\xd1\x13\xc0\x5c\xd2\x93\x27\xcd\xff\xdc\xf8\xe1\x6f\x6e\x2c\xcd\xc5\xee\xdd\xe8\x41\x6a\x31\x81\x69\x70\xde\x64\x0b\x72\x26\xd8\x94\xae\x68\x2d\xb5\xf4\xd2\xe8\x51\x46\x1e\x05\x7a\x9c\x8c\x00\x50\x6b\xe3\x91\xc1\x8e\xff\x02\xfc\xfa\xdb\x08\x40\x63\x46\x13\x90\x79\x6e\x8c\x72\x63\x4d\xfe\xd1\xd8\x87\xf1\x16\xed\x8e\x9c\x27\xbb\x4d\xe5\x58\x9a\x91\xcb\x29\xe5\x4b\x1b\x6b\x42\x3e\x81\xae\x63\x05\xba\x12\x7d\xc1\xda\xfc\xf6\xd6\x18\x15\x01\x4a\x3a\xff\x43\x0d\x78\x2d\x9d\x8f\x1b\xb9\x0a\x16\xd5\x0b\x17\x11\xe6\xb6\xc6\xfa\x9b\x3d\xb6\x84\x77\x55\xed\x67\x79\x4c\xea\x4d\x50\x68\xab\xcb\x23\x00\x97\x9a\x9c\x26\x10\xef\xe6\x98\x92\x18\x01\xec\x0a\x3d\x46\x5c\x09\xa0\x10\x51\x3d\xa8\x6e\xad\xd4\x9e\xec\xd4\xa8\x90\xe9\x17\x4a\x3f\x3b\xa3\x6f\xd1\x6f\x27\x30\x66\xc1\x2b\xad\x30\xc6\x78\xa2\xd2\xda\xcd\xec\xee\xdf\x3f\x2e\x7e\x28\x61\xfe\x99\xc9\x3a\x6f\xa5\xde\xb4\x20\xf2\xe8\x83\x1b\xcb\x7c\xf7\x97\x31\xee\x50\x2a\x5c\xa9\x43\x6c\x97\x1f\x2e\xe7\xd7\x97\xdf\x5e\xcf\x0e\xf0\x31\x7f\x1b\xb2\xfd\x08\x83\x8b\x52\xee\x71\xdd\x2f\x67\x57\x6f\x42\x93\x1a\x5d\xe8\xc4\xfd\xe7\xef\x5f\xfd\x63\xcc\x97\xde\xbf\x3f\x5b\xd0\x46\xb2\x79\x49\x9c\x7d\xfd\xdf\xf2\xe8\x01\x9d\xc5\xec\xbb\xf9\xf2\x6e\xb6\x68\x50\x7b\x45\x09\xed\xc4\xa6\x98\x6e\x69\x41\x28\x9e\x3b\x88\x4d\x2f\xa7\xdf\xcf\x16\xb3\xcb\xab\x9f\x3e\x9d\xd8\xe5\x86\xb4\xef\x23\x76\xf9\xdd\xec\xe6\x6e\x38\xb1\x2a\xd0\xc6\xa9\xa5\x18\x63\x77\x32\x23\xe7\x31\xcb\x9b\x58\x0f\xd0\x09\xf4\x85\x13\x14\xdb\xbb\x77\xa8\xf2\x2d\xbe\x2b\x5c\x3b\xdd\x52\x86\x93\xf2\xbc\xc9\x49\x5f\xde\xce\x3f\x7c\xb3\x3c\x00\x03\xe4\xd6\xe4\x64\xbd\xac\x02\xa5\x58\xb5\xdc\x51\x83\x02\x08\x72\xa9\x95\xb9\x8f\x49\xe5\x63\x72\xb0\x07\xc0\x04\x8a\x5b\x20\x38\x89\x90\x03\xbf\xa5\x2a\x7a\x48\x94\x3c\x81\x59\x83\xdf\x4a\x07\x96\x72\x4b\x8e\x74\x91\x56\x18\x8c\x1a\xcc\xea\x67\x4a\xfd\xb8\x81\x7a\x49\x96\xd1\x70\x5c\x07\x25\x20\x35\x7a\x47\xd6\x83\xa5\xd4\x6c\xb4\xfc\xdf\x0b\x6e\x07\xde\x44\xa2\x0a\x3d\x39\x1f\x1d\xd7\x6a\x54\xb0\x43\x15\xe8\x1c\x50\x8b\x06\xe6\x0c\x9f\xc1\x12\xd3\x84\xa0\x6b\xf8\xe2\x05\xd7\xe4\xe3\x5f\xc6\x12\x48\xbd\x36\x13\xd8\x7a\x9f\xbb\xc9\xc5\xc5\x46\xfa\x2a\xa3\xa6\x26\xcb\x82\x96\xfe\xf9\x22\x35\xda\x5b\xb9\x0a\xde\x58\x77\x21\x68\x47\xea\xc2\xc9\x4d\x82\x36\xdd\x4a\x4f\xa9\x0f\x96\x2e\x30\x97\x49\x14\x44\x47\xff\x1a\x67\xe2\xcf\xb6\xcc\xc1\xee\x80\xec\x91\xef\x14\x2b\x66\xc8\x37\x98\x87\x93\x27\x48\x07\x58\xa2\x2a\x44\xdc\x5b\x81\x41\xac\xba\xc5\x6c\x79\x07\x15\x27\x85\xa5\x0a\xa3\xec\x8f\x1e\xe9\xa5\xb2\x0f\x6b\x53\xea\x35\xd9\xe2\xde\xda\x9a\x2c\xe2\x24\x2d\x72\x23\xb5\x8f\x7f\x52\x25\x49\x7b\x70\x61\x95\x49\xcf\x6e\xf0\x4b\x20\xe7\xd9\x74\x4d\xb4\xd3\xf8\xea\xc0\x8a\x20\xe4\xec\xec\xa2\x79\x60\xae\x61\x8a\x19\xa9\x29\x3a\xfa\x9d\x6d\xc5\x56\x71\x09\x1b\x61\x90\xb5\xea\x6f\x69\xf3\x70\xa1\xde\xda\x46\xf5\x60\xee\x57\x7b\x9c\xf2\xe2\x14\x3e\x35\x7a\x2d\x37\xcd\x9d\xbe\x5b\xbc\x52\x29\x6c\x1b\xbc\x53\x86\xfd\x7a\x4a\x1e\xc2\x8a\xac\x26\x4f\x2e\xd9\xa1\x92\xa2\x5e\x1a\x34\x57\x02\x19\x39\x87\x1b\xce\xc2\xf3\xab\x05\x3b\xa1\xcc\xb2\xe0\x6b\x8f\x58\x73\xd9\xa0\x98\x03\x52\x6b\x78\xff\x1e\x8c\x12\x4b\x52\xeb\x96\xb3\xa2\x8b\xe6\xda\xd8\x0c\xfd\x24\xaa\xa7\xf5\x80\xf4\x94\x75\xdc\x1d\xa0\x80\x0c\x9f\xe6\x11\x01\x7c\xd3\xa3\x41\xb4\x16\x9f\xdb\xb8\x36\x19\x4a\xcd\x15\xc1\x49\xfa\x2f\xae\x2f\x89\x5d\xb4\x1d\xc1\xa7\x09\xd7\xcf\xbc\x22\x74\xc4\x0f\x54\x1f\xef\xf5\x8a\xe1\x70\x69\x9f\x7f\x09\x9e\xf7\x06\x69\x37\x77\xbf\x4c\x5c\xfc\xb5\x93\xee\x0f\x21\x5e\xd4\x4c\xc3\xf5\xf5\xaa\x1b\x0e\x12\x0e\xde\x1c\x72\x70\x18\x76\xb3\x22\xf5\xbf\x16\x75\xf0\x86\xc8\x8b\xa2\x3f\xa5\x2a\x88\x0e\x47\x18\x2c\x7e\xaf\xe1\x61\xa8\x7e\xfa\x0d\x5c\xac\x4f\xd3\x61\x21\xec\x97\xd0\xa3\xf3\x68\xfd\x1f\xde\x89\x96\xcc\xe5\xe7\x17\x9f\xdf\x7f\x69\xa9\x23\x88\x12\x8e\xaf\x8e\x9d\xa8\xb6\x9e\x68\x3f\x7a\x57\xf7\xeb\xe4\x07\xac\x8c\xa4\x8a\x69\x30\x3a\x25\x70\xd4\x4e\xa5\x52\xc3\xd9\x9f\xb6\xe8\xbe\x2a\x95\x30\x2e\xa3\xe6\x6b\xf8\xf8\x11\x18\xee\xea\xc0\xb3\x16\x44\xd6\x04\x4f\x1d\x4f\xf5\xab\xbe\xf1\xe5\xde\xf2\x45\x64\xeb\x73\xbe\xe6\x2e\x96\x91\xf3\xdb\x3f\x9c\xa8\xcb\x92\xb1\xcf\x27\x6c\xb7\xd7\x27\xb1\x30\x6b\x01\xe7\xd5\x28\xe4\x10\x5c\x29\xed\x68\xab\x37\x08\x86\xab\xa2\xd5\xe2\x43\xfc\xbf\xcd\xf7\x0b\x57\x3e\x74\xfd\x12\xd6\xf4\xfc\xda\xdc\xe4\x98\xa9\x0c\x9f\xae\x49\x6f\xb8\x7b\xfe\xeb\xb1\x33\xf4\x3a\xc2\x49\x92\xdf\xec\x99\x79\xcd\x07\x86\xd8\x3f\xc7\xe0\xda\x6c\x5f\x30\xbe\x32\x46\x11\xea\x83\xdd\x76\x7f\x49\xe0\x78\xba\x54\xc7\x74\xdc\x59\xc4\x59\xc5\xd0\xde\x02\x37\xa4\xfd\xad\x11\x0b\x5a\xbf\xb5\xb9\x90\x19\xeb\xed\x94\x30\xd5\xa7\x96\xc5\xba\x9a\xd9\x9d\x74\x3b\xc8\x8e\x17\xa8\xbf\xab\xae\xd6\xfd\xfc\xaa\x68\xae\x99\x0c\xf8\x2d\x7a\xd8\x1a\x25\x1c\x04\x2d\x7f\x09\x04\xf3\xab\x72\xa4\x70\x0e\x52\x73\xaa\xe7\x76\xfb\xfe\x7e\x7e\xe5\xc6\x00\xdf\x52\xca\x0e\x01\x8f\x5d\x39\x45\x18\x7d\xe6\xe1\xc7\x9b\xeb\x9f\x80\xcf\xc5\x7b\xe7\x45\x8f\xcd\x44\x35\xa0\x92\xc5\xf0\xa3\x90\x2f\xe2\x64\x0a\x25\x3f\x29\xe6\xdc\xc7\xba\x0e\xf4\x5c\xad\x6b\x0f\xa8\x05\x6c\x49\xe5\xdc\xc7\x3f\x10\xb8\x60\x4b\x49\x98\x5c\xdc\x8d\x2a\x06\x61\x80\xdb\xf2\x0d\x79\x48\x8d\x5e\xab\xb6\xce\x7c\x80\xce\x7b\xf2\xd3\x7e\xec\x76\x6c\x93\xce\x72\xf1\xb5\x52\x5d\xa1\xf3\x77\x16\xb5\x93\xd5\x88\xad\xab\xec\x39\x30\xf9\x35\x3a\x0f\x5e\x66\x54\x0c\x2f\x2a\xce\xc0\xbf\xa0\x22\x51\x4c\x3a\x8c\x26\x38\x18\x06\xb6\x28\xc4\x00\x6a\xe3\xb7\x64\xdb\x15\x36\xe0\x11\x63\x31\xee\xe3\x38\x64\xb0\x08\x77\x71\x22\xb6\x17\x43\xba\x9a\x1c\x8f\xe8\xba\xc6\x2b\x83\x79\xaa\xf2\xe4\x10\x66\xbe\x0f\x19\xea\xc4\x12\x0a\x4e\xa0\xd5\x55\x90\x5a\xc8\x14\xe3\x14\x4a\x90\x47\xa9\x1c\xe0\xca\x84\xae\xc2\x0a\x4a\x81\x5e\x8c\x70\x2a\xeb\x96\xd0\x35\xe7\x9c\x1d\x9c\xb3\x1a\x8b\xe3\x5c\x8b\x1c\xba\xc3\x99\x6b\x32\x74\xb2\x32\xdb\x72\x74\x07\x47\xcb\x78\xb4\x98\xa8\xd6\x98\x39\x8f\xae\x68\xd6\x70\x67\x03\x9d\xc3\x3f\x51\x39\x3a\x87\x7b\xfd\xa0\xcd\xe3\xe9\x7c\xc5\x03\x83\xf4\xc4\x29\xc7\xac\x21\x55\xc1\x71\xbd\xf0\xc2\xd7\x89\xa4\xfb\x3a\x84\xa4\x3b\xe2\x92\x88\xb7\x65\xa3\xb7\x30\xea\xee\x23\xb9\xee\x7c\xeb\x33\x88\x4a\x99\x94\x43\xab\x5d\x71\xf5\x6f\x49\xaf\xcd\x19\x06\x8e\x6d\x3a\x9b\x9e\x97\xef\x46\xa7\xcd\x6d\xda\x0b\x96\xd7\x6f\xf6\x15\xba\xcd\x4f\x59\xf5\xbd\xda\x57\xa9\x41\x22\xee\xd3\xe2\x31\xa5\xaa\x73\xe0\xdd\x84\x73\xe0\xf0\x8a\xb1\x95\xe2\x11\x30\xd6\xe0\x62\x02\xde\x86\x02\xb7\xf3\xc6\xc6\xc2\x71\x0f\x09\xab\x97\xf1\x7e\xc5\x61\x19\xe9\xf0\xeb\x6f\xa3\xff\x07\x00\x00\xff\xff\x51\x01\x92\x53\xb3\x1d\x00\x00")

func chartCrdsNetworkHarvesterhciIo_ippoolsYamlBytes() ([]byte, error) {
	return bindataRead(
		_chartCrdsNetworkHarvesterhciIo_ippoolsYaml,
		"chart/crds/network.harvesterhci.io_ippools.yaml",
	)
}

func chartCrdsNetworkHarvesterhciIo_ippoolsYaml() (*asset, error) {
	bytes, err := chartCrdsNetworkHarvesterhciIo_ippoolsYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "chart/crds/network.harvesterhci.io_ippools.yaml", size: 7603, mode: os.FileMode(420), modTime: time.Unix(1739258711, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _chartCrdsNetworkHarvesterhciIo_virtualmachinenetworkconfigsYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xbc\x57\xdd\x6f\x23\x35\x10\x7f\xcf\x5f\x31\x12\x0f\x07\xd2\x6d\xa2\x0a\x04\x28\x52\x05\x21\x3d\x20\xa2\x2d\xd5\xa5\x57\x09\x21\x1e\x26\xeb\x49\xe2\xab\x3f\x16\x7b\x9c\xbb\x72\xdc\xff\x8e\x6c\xef\x36\x9b\x8f\x4d\x73\xa1\xe0\xb7\xb5\x67\xe6\x37\xdf\x33\x5b\x14\x45\x0f\x2b\x79\x47\xce\x4b\x6b\x86\x80\x95\xa4\xf7\x4c\x26\x7e\xf9\xfe\xfd\xb7\xbe\x2f\xed\x60\x75\xd6\xbb\x97\x46\x0c\x61\x1c\x3c\x5b\xfd\x9a\xbc\x0d\xae\xa4\x0b\x9a\x4b\x23\x59\x5a\xd3\xd3\xc4\x28\x90\x71\xd8\x03\x40\x63\x2c\x63\xbc\xf6\xf1\x13\xe0\xc3\xc7\x1e\x80\x41\x4d\x43\x58\x49\xc7\x01\x95\xc6\x72\x29\x0d\x19\xe2\x77\xd6\xdd\x97\xd6\xcc\xe5\xc2\xf7\xeb\xcf\xfe\x12\xdd\x8a\x3c\x93\x5b\x96\xb2\x2f\x6d\xcf\x57\x54\x46\x49\x0b\x67\x43\x35\x84\x2e\xb2\x8c\x51\x63\x66\x7d\xef\x32\xdc\x55\x86\xbb\xce\x8c\xe3\x04\x97\xa8\x94\xf4\xfc\xcb\x53\x94\x97\xd2\x73\xa2\xae\x54\x70\xa8\x0e\x1b\x91\x08\xfd\xd2\x3a\xbe\x5e\x2b\x53\xc0\x4a\x1b\xe2\x72\xbe\xd8\xfa\xac\xc9\xa5\x59\x04\x85\xee\xa0\xe4\x1e\x80\x2f\x6d\x45\x43\x48\x82\x2b\x2c\x49\xf4\x00\x56\x39\x70\x09\xa8\x00\x14\x22\xc5\x03\xd5\x8d\x93\x86\xc9\x8d\xad\x0a\xda\x3c\xaa\xf1\xd6\x5b\x73\x83\xbc\x1c\x42\x3f\x3a\xb5\xbf\xd2\x51\x58\x7a\x6c\x22\x74\x77\x75\x3d\xba\x7a\x55\x5f\xf1\x43\x04\xf4\xec\xa4\x59\xec\x11\xc1\xc8\xc1\xf7\x4b\x6b\x32\xaa\xff\xfd\xbb\xcf\xbf\xef\x47\x9e\xf3\xf3\x17\x23\xa5\x6c\x89\x4c\xe2\xc5\x17\x7f\xd4\x94\x1b\x38\xa3\xcb\xcb\x5f\xc7\xa3\xdb\x57\x17\xff\x1e\xea\x42\x7a\x9c\xa9\x4e\xa4\x8b\xc9\x74\xf4\xc3\xe5\x73\x00\x4d\xcc\xf4\xc1\x94\x9d\x40\x93\xeb\xe9\x6f\xd7\xe3\x23\x81\x9a\x8a\xe9\x97\x8e\x52\xb1\xdc\x4a\x4d\x9e\x51\x57\x9b\x6e\xfa\x69\x33\x16\x02\x39\xc7\xab\xae\xa7\x33\x54\xd5\x12\xcf\x72\x1e\x95\x4b\xd2\x38\xac\xe9\x6d\x45\x66\x74\x33\xb9\xfb\x72\xba\x71\x0d\x50\x39\x5b\x91\x63\xd9\x64\x67\x3e\xad\x26\xd0\xba\x05\x10\xe4\x4b\x27\x2b\x4e\xdd\xe1\xef\x62\xe3\x0d\x20\x02\x64\x2e\x10\xb1\x1b\x90\x07\x5e\x52\x93\x95\x24\x6a\x9d\xc0\xce\x81\x97\xd2\x83\xa3\xca\x91\x27\x93\xfb\x43\xbc\x46\x03\x76\xf6\x96\x4a\xee\x6f\x89\x9e\x92\x8b\x62\x62\x31\x05\x25\xa0\xb4\x66\x45\x8e\xc1\x51\x69\x17\x46\xfe\xf5\x28\xdb\x03\xdb\x04\xaa\x90\xc9\x33\xa4\xbc\x37\xa8\x60\x85\x2a\xd0\x4b\x40\x23\xb6\x24\x6b\x7c\x00\x47\x11\x13\x82\x69\xc9\x4b\x0c\x7e\x5b\x8f\x2b\xeb\x08\xa4\x99\xdb\x21\x2c\x99\x2b\x3f\x1c\x0c\x16\x92\x9b\xd6\x58\x5a\xad\x83\x91\xfc\x30\x28\xad\x61\x27\x67\x81\xad\xf3\x03\x41\x2b\x52\x03\x2f\x17\x05\xba\x72\x29\x99\x4a\x0e\x8e\x06\x58\xc9\x22\x19\x62\x52\x6e\xf5\xb5\xf8\xcc\xd5\xcd\xd4\x6f\xc0\xee\xe4\x4e\x3e\xa9\xab\x7d\x42\x78\x62\x6f\x03\xe9\x01\x6b\x51\xd9\xc4\x75\x14\xe2\x55\x74\xdd\xeb\x57\xd3\x5b\x68\x34\xc9\x91\xca\x41\x59\x93\xee\xf8\xa5\x89\x4f\xf4\xa6\x34\x73\x72\x99\x6f\xee\xac\x4e\x32\xc9\x88\xca\x4a\xc3\xe9\xa3\x54\x92\x0c\x83\x0f\x33\x2d\x39\xa6\xc1\x9f\x81\x3c\xc7\xd0\x6d\x8b\x1d\xa7\xf1\x01\x33\x82\x50\xc5\x64\x17\xdb\x04\x13\x03\x63\xd4\xa4\xc6\xe8\xe9\x7f\x8e\x55\x8c\x8a\x2f\x62\x10\x8e\x8a\x56\x7b\x28\x6e\x13\x67\xf7\xb6\x1e\x9a\x21\xb7\x3e\xfb\xeb\x34\x1e\xd3\x1e\x4f\x3b\xaf\x00\x92\x49\xef\xb9\x3e\x24\xb2\x66\xac\x46\x42\x38\xf2\x1d\xcf\x00\x73\xeb\x34\xf2\x10\x64\xb5\xfa\xaa\x83\xa4\xc3\x19\xeb\xa3\xb1\x7c\x02\x45\xe3\xfb\x4b\x32\x8b\xd8\x27\xcf\xbe\x39\x15\xa6\x76\x52\x1c\x70\x47\xe0\x7c\x7d\xa2\x39\x31\x93\xa5\x23\xb1\x0f\xa2\x68\x99\xba\xf7\xb9\xa5\xe2\x9e\xf7\x8e\x44\x79\x54\x7d\x92\xa2\x0c\xbb\x8a\x67\x46\x74\x0e\x1f\xb6\xde\x2a\x0c\x7e\x9f\xae\x99\x63\x66\xad\x22\x34\x5b\xaf\x79\x47\xd8\xe5\x39\xec\xbc\x83\x6e\x7b\x5f\xdc\x87\x19\x39\x43\x4c\xbe\x58\xa1\x92\xa2\xbd\x2e\x6e\xfa\x48\x93\xf7\xb8\xc8\x8b\x09\x6a\x8a\xdd\x4c\x6a\x1d\x38\x4e\xfc\x7d\xf1\x08\x2a\xe2\x92\x9a\xc3\xf9\x39\x58\x25\xa6\xa4\xe6\x1b\x74\xfb\x23\x56\xc0\xc6\x2e\xd4\x36\x62\xb7\x54\xd3\xe8\x3f\xb6\x58\xd7\xab\xc4\x33\x16\xaa\x42\xcf\xb7\x0e\x8d\x97\xcd\xea\xd0\x95\xe3\x1b\x03\xe2\x12\x3d\x03\x4b\x4d\xb9\x29\x37\x9a\x01\x3f\x8a\x22\x91\x3b\xb8\x35\x04\x1b\x2b\xce\xee\x61\x0b\x68\x2c\x2f\xc9\x6d\xb7\xe1\x47\x8a\xa7\x6a\x34\x9a\xf1\x26\xb5\xf9\xa3\x4d\xb8\x4d\x93\x7e\x6d\x86\xf4\x2d\x3b\xde\xa1\xef\x1a\x1b\x47\xeb\xd4\x24\xdc\x31\xca\xfc\x1c\x34\x9a\xc2\x11\x8a\x98\x8e\x0d\x2b\x48\x23\x64\x89\x69\xba\x0a\x62\x94\xca\x03\xce\x6c\xd8\xad\xe2\xb6\x1f\x5a\x41\x38\x55\x75\x47\xe8\xb7\xf7\xb7\x0e\xcd\xa3\x1b\x33\x79\xec\xe9\x9b\xe9\xf0\xc2\x6f\x2b\x74\xb2\x33\xf7\x95\x4a\x87\x46\xd3\x44\x9a\x37\xc5\x96\x32\x2f\x53\x2a\xda\x39\xdc\xba\xb8\xcd\xfd\x88\xca\xd3\x4b\x78\x63\xee\x8d\x7d\x77\xba\x5e\x89\xe0\x28\x3f\x3d\x54\x09\xbd\x54\x21\xfe\x6a\xae\xf5\x3a\x11\xfa\xf0\xbc\xe8\xac\xb8\x22\xc9\xfd\xd4\x21\xd1\x3d\x08\xfe\xb3\x0d\x02\x9b\xdf\xbe\xc9\xcd\x13\x43\xfe\x19\xf6\x84\xe7\xd8\x01\x8e\x4a\xe1\x53\xb9\x4f\x8a\xce\x5e\xa6\x9d\x4b\x1f\xd7\x6f\x31\x04\x76\x21\xe7\x85\x67\xeb\xd2\xa0\x5c\xdf\x84\xd9\xe3\xdf\x45\x63\x40\x5d\x90\xf0\xe1\x63\xef\x9f\x00\x00\x00\xff\xff\xaa\xdd\x1c\xed\xfb\x11\x00\x00")

func chartCrdsNetworkHarvesterhciIo_virtualmachinenetworkconfigsYamlBytes() ([]byte, error) {
	return bindataRead(
		_chartCrdsNetworkHarvesterhciIo_virtualmachinenetworkconfigsYaml,
		"chart/crds/network.harvesterhci.io_virtualmachinenetworkconfigs.yaml",
	)
}

func chartCrdsNetworkHarvesterhciIo_virtualmachinenetworkconfigsYaml() (*asset, error) {
	bytes, err := chartCrdsNetworkHarvesterhciIo_virtualmachinenetworkconfigsYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "chart/crds/network.harvesterhci.io_virtualmachinenetworkconfigs.yaml", size: 4603, mode: os.FileMode(420), modTime: time.Unix(1739258711, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"chart/crds/network.harvesterhci.io_ippools.yaml":                      chartCrdsNetworkHarvesterhciIo_ippoolsYaml,
	"chart/crds/network.harvesterhci.io_virtualmachinenetworkconfigs.yaml": chartCrdsNetworkHarvesterhciIo_virtualmachinenetworkconfigsYaml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"chart": &bintree{nil, map[string]*bintree{
		"crds": &bintree{nil, map[string]*bintree{
			"network.harvesterhci.io_ippools.yaml":                      &bintree{chartCrdsNetworkHarvesterhciIo_ippoolsYaml, map[string]*bintree{}},
			"network.harvesterhci.io_virtualmachinenetworkconfigs.yaml": &bintree{chartCrdsNetworkHarvesterhciIo_virtualmachinenetworkconfigsYaml, map[string]*bintree{}},
		}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
