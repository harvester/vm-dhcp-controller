package ippool

import (
	"github.com/harvester/webhook/pkg/server/admission"
	"github.com/sirupsen/logrus"
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"

	networkv1 "github.com/harvester/vm-dhcp-controller/pkg/apis/network.harvesterhci.io/v1alpha1"
)

type Validator struct {
	admission.DefaultValidator
}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) Create(request *admission.Request, newObj runtime.Object) error {
	ipPool := newObj.(*networkv1.IPPool)
	logrus.Infof("create ippool %s/%s", ipPool.Namespace, ipPool.Name)
	return nil
}

func (v *Validator) Delete(request *admission.Request, newObj runtime.Object) error {
	ipPool := newObj.(*networkv1.IPPool)
	logrus.Infof("delete ippool %s/%s", ipPool.Namespace, ipPool.Name)
	return nil
}

func (v *Validator) Resource() admission.Resource {
	return admission.Resource{
		Names:      []string{"ippools"},
		Scope:      admissionregv1.NamespacedScope,
		APIGroup:   networkv1.SchemeGroupVersion.Group,
		APIVersion: networkv1.SchemeGroupVersion.Version,
		ObjectType: &networkv1.IPPool{},
		OperationTypes: []admissionregv1.OperationType{
			admissionregv1.Create,
		},
	}
}
