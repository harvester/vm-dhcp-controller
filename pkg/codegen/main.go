package main

import (
	"bytes"
	"os"
	"path/filepath"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	// Ensure gvk gets loaded in wrangler/pkg/gvk cache
	_ "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io/v1"
)

func nadControllerInterfaceRefactor() {
	absPath, _ := filepath.Abs("pkg/generated/controllers/k8s.cni.cncf.io/v1/interface.go")
	input, err := os.ReadFile(absPath)
	if err != nil {
		logrus.Fatalf("failed to read the network-attachment-definition file: %v", err)
	}

	output := bytes.Replace(input, []byte("networkattachmentdefinitions"), []byte("network-attachment-definitions"), -1)

	if err = os.WriteFile(absPath, output, 0644); err != nil {
		logrus.Fatalf("failed to update the network-attachment-definition file: %v", err)
	}
}

func main() {
	os.Unsetenv("GOPATH")
	controllergen.Run(args.Options{
		OutputPackage: "github.com/starbops/vm-dhcp-controller/pkg/generated",
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
					corev1.Pod{},
				},
				InformersPackage: "k8s.io/client-go/informers",
				ClientSetPackage: "k8s.io/client-go/kubernetes",
				ListersPackage:   "k8s.io/client-go/listers",
			},
			cniv1.SchemeGroupVersion.Group: {
				Types: []interface{}{
					cniv1.NetworkAttachmentDefinition{},
				},
				GenerateTypes:   false,
				GenerateClients: true,
			},
			kubevirtv1.SchemeGroupVersion.Group: {
				Types: []interface{}{
					kubevirtv1.VirtualMachine{},
				},
				GenerateTypes:   false,
				GenerateClients: true,
			},
		},
	})

	nadControllerInterfaceRefactor()
}
