/*
Copyright The Kubernetes Authors.

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

package controllers

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1exp "sigs.k8s.io/cluster-api-provider-gcp/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGCPManagedMachinePoolReconciler_PersistsConditionsWhenControlPlaneNotReady(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	scheme := runtime.NewScheme()
	g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
	g.Expect(infrav1exp.AddToScheme(scheme)).To(Succeed())

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-cluster", Namespace: "default"},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrav1exp.GroupVersion.Group,
				Kind:     "GCPManagedCluster",
				Name:     "my-managed-cluster",
			},
			ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrav1exp.GroupVersion.Group,
				Kind:     "GCPManagedControlPlane",
				Name:     "my-control-plane",
			},
		},
	}
	machinePool := &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-machine-pool",
			Namespace: "default",
			Labels:    map[string]string{clusterv1.ClusterNameLabel: cluster.Name},
		},
	}
	gcpManagedMachinePool := &infrav1exp.GCPManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-gcp-machine-pool",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: clusterv1.GroupVersion.String(),
				Kind:       "MachinePool",
				Name:       machinePool.Name,
			}},
		},
	}
	gcpManagedCluster := &infrav1exp.GCPManagedCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-managed-cluster", Namespace: "default"},
	}
	gcpManagedControlPlane := &infrav1exp.GCPManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "my-control-plane", Namespace: "default"},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster, machinePool, gcpManagedMachinePool, gcpManagedCluster, gcpManagedControlPlane).
		WithStatusSubresource(&infrav1exp.GCPManagedMachinePool{}).
		Build()
	r := &GCPManagedMachinePoolReconciler{Client: c}

	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(gcpManagedMachinePool)})
	g.Expect(err).NotTo(HaveOccurred())

	got := &infrav1exp.GCPManagedMachinePool{}
	g.Expect(c.Get(ctx, client.ObjectKeyFromObject(gcpManagedMachinePool), got)).To(Succeed())

	g.Expect(v1beta1conditions.IsFalse(got, infrav1exp.GKEMachinePoolReadyCondition)).To(BeTrue())
	g.Expect(v1beta1conditions.GetReason(got, infrav1exp.GKEMachinePoolReadyCondition)).To(Equal(infrav1exp.WaitingForGKEControlPlaneReason))
	for _, conditionType := range []string{infrav1exp.GCPManagedMachinePoolReadyCondition, infrav1exp.GCPManagedMachinePoolGKEMachinePoolReadyCondition} {
		condition := v1beta2conditions.Get(got, conditionType)
		g.Expect(condition).NotTo(BeNil(), conditionType)
		g.Expect(condition.Status).To(Equal(metav1.ConditionFalse), conditionType)
		g.Expect(condition.Reason).To(Equal(infrav1exp.WaitingForGKEControlPlaneReason), conditionType)
	}
}
