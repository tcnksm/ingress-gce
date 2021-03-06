/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backendconfig

import (
	"errors"

	"github.com/golang/glog"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"k8s.io/ingress-gce/pkg/annotations"
	apisbackendconfig "k8s.io/ingress-gce/pkg/apis/backendconfig"
	backendconfigv1beta1 "k8s.io/ingress-gce/pkg/apis/backendconfig/v1beta1"
	"k8s.io/ingress-gce/pkg/crd"
)

var (
	ErrBackendConfigDoesNotExist = errors.New("no BackendConfig for service port exists.")
	ErrBackendConfigFailedToGet  = errors.New("client had error getting BackendConfig for service port.")
	ErrNoBackendConfigForPort    = errors.New("no BackendConfig name found for service port.")
)

func CRDMeta() *crd.CRDMeta {
	meta := crd.NewCRDMeta(
		apisbackendconfig.GroupName,
		"v1beta1",
		"BackendConfig",
		"BackendConfigList",
		"backendconfig",
		"backendconfigs",
	)
	meta.AddValidationInfo("k8s.io/ingress-gce/pkg/apis/backendconfig/v1beta1.BackendConfig", backendconfigv1beta1.GetOpenAPIDefinitions)
	return meta
}

// GetServicesForBackendConfig returns all services that reference the given
// BackendConfig.
func GetServicesForBackendConfig(svcLister cache.Store, backendConfig *backendconfigv1beta1.BackendConfig) []*apiv1.Service {
	svcs := []*apiv1.Service{}
	for _, obj := range svcLister.List() {
		svc := obj.(*apiv1.Service)
		if svc.Namespace != backendConfig.Namespace {
			continue
		}
		backendConfigNames, err := annotations.FromService(svc).GetBackendConfigs()
		if err != nil {
			// If the user did not provide the annotation at all, then we
			// do not want to log an error.
			if err != annotations.ErrBackendConfigAnnotationMissing {
				glog.Errorf("Failed to get BackendConfig names from service %s/%s: %v", svc.Namespace, svc.Name, err)
			}
			continue
		}
		if backendConfigNames != nil {
			if backendConfigNames.Default == backendConfig.Name {
				svcs = append(svcs, svc)
				continue
			}
			for _, backendConfigName := range backendConfigNames.Ports {
				if backendConfigName == backendConfig.Name {
					svcs = append(svcs, svc)
					break
				}
			}
		}
	}
	return svcs
}

// GetBackendConfigForServicePort returns the corresponding BackendConfig for
// the given ServicePort if specified.
func GetBackendConfigForServicePort(backendConfigLister cache.Store, svc *apiv1.Service, svcPort *apiv1.ServicePort) (*backendconfigv1beta1.BackendConfig, error) {
	backendConfigs, err := annotations.FromService(svc).GetBackendConfigs()
	if err != nil {
		// If the user did not provide the annotation at all, then we
		// do not want to return an error.
		if err == annotations.ErrBackendConfigAnnotationMissing {
			return nil, nil
		}
		return nil, err
	}

	configName := BackendConfigName(*backendConfigs, *svcPort)
	if configName == "" {
		return nil, ErrNoBackendConfigForPort
	}

	obj, exists, err := backendConfigLister.Get(
		&backendconfigv1beta1.BackendConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configName,
				Namespace: svc.Namespace,
			},
		})
	if err != nil {
		return nil, ErrBackendConfigFailedToGet
	}
	if !exists {
		return nil, ErrBackendConfigDoesNotExist
	}

	return obj.(*backendconfigv1beta1.BackendConfig), nil
}
