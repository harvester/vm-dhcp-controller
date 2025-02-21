package main

import (
	"testing"

	"github.com/harvester/vm-dhcp-controller/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestParseImageNameAndTag(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected *config.Image
		err      bool
	}{
		{
			name:  "valid image with registry and tag",
			image: "myregistry.local:5000/rancher/harvester-vm-dhcp-controller:v0.3.3",
			expected: &config.Image{
				Repository: "myregistry.local:5000/rancher/harvester-vm-dhcp-controller",
				Tag:        "v0.3.3",
			},
			err: false,
		},
		{
			name:  "valid image with only image and tag",
			image: "rancher/harvester-vm-dhcp-controller:v0.3.3",
			expected: &config.Image{
				Repository: "rancher/harvester-vm-dhcp-controller",
				Tag:        "v0.3.3",
			},
			err: false,
		},
		{
			name:  "valid image without tag",
			image: "rancher/harvester-vm-dhcp-controller",
			expected: &config.Image{
				Repository: "rancher/harvester-vm-dhcp-controller",
				Tag:        "latest",
			},
			err: false,
		},
		{
			name:  "valid image with port but no tag",
			image: "myregistry.local:5000/rancher/harvester-vm-dhcp-controller",
			expected: &config.Image{
				Repository: "myregistry.local:5000/rancher/harvester-vm-dhcp-controller",
				Tag:        "latest",
			},
			err: false,
		},
		{
			name:     "invalid image with colon but no tag",
			image:    "myregistry.local:5000/rancher/harvester-vm-dhcp-controller:",
			expected: nil,
			err:      true,
		},
		{
			name:     "invalid image with multiple colons",
			image:    "myregistry.local:5000/rancher/harvester-vm-dhcp-controller:v0.3.3:latest",
			expected: nil,
			err:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseImageNameAndTag(tt.image)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
