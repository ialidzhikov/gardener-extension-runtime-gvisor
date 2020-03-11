// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"

	"github.com/gardener/gardener-extension-runtime-gvisor/pkg/charts"
	extensionscontroller "github.com/gardener/gardener-extensions/pkg/controller"
	"github.com/gardener/gardener-resource-manager/pkg/manager"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/common"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	GVisorRuntimeSecretName = "extension-runtime-gvisor-config"
)

// Reconcile implements Network.Actuator.
func (a *actuator) Reconcile(ctx context.Context, cr *extensionsv1alpha1.ContainerRuntime, cluster *extensionscontroller.Cluster) error {

	// Create shoot chart renderer
	chartRenderer, err := a.chartRendererFactory.NewChartRendererForShoot(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return errors.Wrapf(err, "could not create chart renderer for shoot '%s'", cr.Namespace)
	}

	calicoChart, err := charts.RenderGVisorChart(chartRenderer, cr)
	if err != nil {
		return err
	}

	secret, secretRefs := gvisorSecret(a.client, calicoChart, cr.Namespace)
	if err := secret.Reconcile(ctx); err != nil {
		return err
	}

	return manager.
		NewManagedResource(a.client).
		WithNamespacedName(cr.Namespace, GVisorRuntimeSecretName).
		WithSecretRefs(secretRefs).
		WithInjectedLabels(map[string]string{common.ShootNoCleanup: "true"}).
		Reconcile(ctx)
}

func withLocalObjectRefs(refs ...string) []corev1.LocalObjectReference {
	var localObjectRefs []corev1.LocalObjectReference
	for _, ref := range refs {
		localObjectRefs = append(localObjectRefs, corev1.LocalObjectReference{Name: ref})
	}
	return localObjectRefs
}

func gvisorSecret(cl client.Client, gvisorConfig []byte, namespace string) (*manager.Secret, []corev1.LocalObjectReference) {
	return manager.NewSecret(cl).
		WithKeyValues(map[string][]byte{charts.GVisorConfigKey: gvisorConfig}).
		WithNamespacedName(namespace, GVisorRuntimeSecretName), withLocalObjectRefs(GVisorRuntimeSecretName)
}

