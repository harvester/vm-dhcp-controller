package main

import (
	"bytes"
	"os"
	"path/filepath"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	controllergen "github.com/rancher/wrangler/v3/pkg/controller-gen"
	"github.com/rancher/wrangler/v3/pkg/controller-gen/args"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"
)

func nadControllerInterfaceRefactor() {
	absPath, _ := filepath.Abs("pkg/generated/controllers/k8s.cni.cncf.io/v1/interface.go")
	input, err := os.ReadFile(absPath)
	if err != nil {
		logrus.Fatalf("failed to read the network-attachment-definition file: %v", err)
	}

	output := bytes.ReplaceAll(input, []byte("networkattachmentdefinitions"), []byte("network-attachment-definitions"))

	if err = os.WriteFile(absPath, output, 0644); err != nil {
		logrus.Fatalf("failed to update the network-attachment-definition file: %v", err)
	}
}

func main() {
	if err := os.Unsetenv("GOPATH"); err != nil {
		logrus.Fatalf("failed to unset GOPATH: %v", err)
	}
	controllergen.Run(args.Options{
		OutputPackage: "github.com/harvester/vm-dhcp-controller/pkg/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"network.harvesterhci.io": {
				Types: []interface{}{
					"./pkg/apis/network.harvesterhci.io/v1alpha1",
				},
				GenerateTypes:   true,
				GenerateClients: true,
			},
			corev1.GroupName: {
				Types: []interface{}{
					corev1.Node{},
					corev1.Pod{},
				},
			},
			appsv1.SchemeGroupVersion.Group: {
				Types: []interface{}{
					appsv1.Deployment{},
				},
			},
			cniv1.SchemeGroupVersion.Group: {
				Types: []interface{}{
					cniv1.NetworkAttachmentDefinition{},
				},
				GenerateClients: true,
			},
			kubevirtv1.SchemeGroupVersion.Group: {
				Types: []interface{}{
					kubevirtv1.VirtualMachine{},
				},
				GenerateClients: true,
			},
		},
	})

	nadControllerInterfaceRefactor()
}
