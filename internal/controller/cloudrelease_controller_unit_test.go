/*
Copyright 2026.

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

package controller

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	deliveryv1alpha1 "github.com/senak1129/cloudrelease-operator/api/v1alpha1"
)

func TestReconcileCreatesRepairsAndReportsResources(t *testing.T) {
	g := gomega.NewWithT(t)
	ctx := context.Background()
	release := unitTestCloudRelease()
	reconciler, k8sClient := newUnitTestReconciler(t, release)
	request := ctrl.Request{NamespacedName: types.NamespacedName{
		Name:      release.Name,
		Namespace: release.Namespace,
	}}

	_, err := reconciler.Reconcile(ctx, request)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	deployment := &appsv1.Deployment{}
	g.Expect(k8sClient.Get(ctx, request.NamespacedName, deployment)).To(gomega.Succeed())
	g.Expect(deployment.Spec.Replicas).NotTo(gomega.BeNil())
	g.Expect(*deployment.Spec.Replicas).To(gomega.Equal(int32(2)))
	g.Expect(deployment.Spec.Template.Spec.Containers).To(gomega.HaveLen(1))
	g.Expect(deployment.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal("nginx:1.27-alpine"))
	g.Expect(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort).To(gomega.Equal(int32(80)))
	g.Expect(deployment.Spec.Selector.MatchLabels[releaseUIDLabel]).To(gomega.Equal(string(release.UID)))
	g.Expect(metav1.IsControlledBy(deployment, release)).To(gomega.BeTrue())

	service := &corev1.Service{}
	g.Expect(k8sClient.Get(ctx, request.NamespacedName, service)).To(gomega.Succeed())
	g.Expect(service.Spec.Type).To(gomega.Equal(corev1.ServiceTypeClusterIP))
	g.Expect(service.Spec.Selector[releaseUIDLabel]).To(gomega.Equal(string(release.UID)))
	g.Expect(service.Spec.Ports).To(gomega.HaveLen(1))
	g.Expect(service.Spec.Ports[0].Port).To(gomega.Equal(int32(80)))
	g.Expect(service.Spec.Ports[0].TargetPort).To(gomega.Equal(intstr.FromString("http")))
	g.Expect(metav1.IsControlledBy(service, release)).To(gomega.BeTrue())

	currentRelease := &deliveryv1alpha1.CloudRelease{}
	g.Expect(k8sClient.Get(ctx, request.NamespacedName, currentRelease)).To(gomega.Succeed())
	g.Expect(currentRelease.Status.ObservedGeneration).To(gomega.Equal(int64(1)))
	g.Expect(currentRelease.Status.ReadyReplicas).To(gomega.Equal(int32(0)))
	g.Expect(meta.FindStatusCondition(currentRelease.Status.Conditions, conditionAvailable).Status).
		To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(meta.FindStatusCondition(currentRelease.Status.Conditions, conditionProgressing).Status).
		To(gomega.Equal(metav1.ConditionTrue))

	deploymentResourceVersion := deployment.ResourceVersion
	serviceResourceVersion := service.ResourceVersion
	_, err = reconciler.Reconcile(ctx, request)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(k8sClient.Get(ctx, request.NamespacedName, deployment)).To(gomega.Succeed())
	g.Expect(k8sClient.Get(ctx, request.NamespacedName, service)).To(gomega.Succeed())
	g.Expect(deployment.ResourceVersion).To(gomega.Equal(deploymentResourceVersion))
	g.Expect(service.ResourceVersion).To(gomega.Equal(serviceResourceVersion))

	driftedReplicas := int32(1)
	deployment.Spec.Replicas = &driftedReplicas
	g.Expect(k8sClient.Update(ctx, deployment)).To(gomega.Succeed())

	service.Spec.ClusterIP = "10.96.0.42"
	g.Expect(k8sClient.Update(ctx, service)).To(gomega.Succeed())

	_, err = reconciler.Reconcile(ctx, request)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(k8sClient.Get(ctx, request.NamespacedName, deployment)).To(gomega.Succeed())
	g.Expect(*deployment.Spec.Replicas).To(gomega.Equal(int32(2)))
	g.Expect(k8sClient.Get(ctx, request.NamespacedName, service)).To(gomega.Succeed())
	g.Expect(service.Spec.ClusterIP).To(gomega.Equal("10.96.0.42"))

	g.Expect(k8sClient.Delete(ctx, service)).To(gomega.Succeed())
	_, err = reconciler.Reconcile(ctx, request)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(k8sClient.Get(ctx, request.NamespacedName, service)).To(gomega.Succeed())

	g.Expect(k8sClient.Get(ctx, request.NamespacedName, deployment)).To(gomega.Succeed())
	deployment.Status.ObservedGeneration = deployment.Generation
	deployment.Status.UpdatedReplicas = 2
	deployment.Status.ReadyReplicas = 2
	deployment.Status.AvailableReplicas = 2
	deployment.Status.UnavailableReplicas = 0
	g.Expect(k8sClient.Status().Update(ctx, deployment)).To(gomega.Succeed())

	_, err = reconciler.Reconcile(ctx, request)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(k8sClient.Get(ctx, request.NamespacedName, currentRelease)).To(gomega.Succeed())
	g.Expect(currentRelease.Status.ReadyReplicas).To(gomega.Equal(int32(2)))
	g.Expect(meta.FindStatusCondition(currentRelease.Status.Conditions, conditionAvailable).Status).
		To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(meta.FindStatusCondition(currentRelease.Status.Conditions, conditionProgressing).Status).
		To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(meta.FindStatusCondition(currentRelease.Status.Conditions, conditionDegraded).Status).
		To(gomega.Equal(metav1.ConditionFalse))
}

func TestReconcileRefusesUnownedDeployment(t *testing.T) {
	g := gomega.NewWithT(t)
	ctx := context.Background()
	release := unitTestCloudRelease()
	foreignDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      release.Name,
			Namespace: release.Namespace,
		},
	}
	reconciler, _ := newUnitTestReconciler(t, release, foreignDeployment)

	_, _, err := reconciler.reconcileDeployment(ctx, release)
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring(
		"already exists and is not controlled by CloudRelease",
	)))
}

func TestReconcileReturnsSuccessWhenCloudReleaseDoesNotExist(t *testing.T) {
	g := gomega.NewWithT(t)
	ctx := context.Background()
	reconciler, k8sClient := newUnitTestReconciler(t)
	request := ctrl.Request{NamespacedName: types.NamespacedName{
		Name:      "missing",
		Namespace: "default",
	}}

	result, err := reconciler.Reconcile(ctx, request)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(result).To(gomega.Equal(ctrl.Result{}))

	missing := &deliveryv1alpha1.CloudRelease{}
	err = k8sClient.Get(ctx, request.NamespacedName, missing)
	g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
}

func newUnitTestReconciler(
	t *testing.T,
	objects ...client.Object,
) (*CloudReleaseReconciler, client.Client) {
	t.Helper()

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add Kubernetes types to test scheme: %v", err)
	}
	if err := deliveryv1alpha1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add CloudRelease types to test scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(testScheme).
		WithStatusSubresource(
			&deliveryv1alpha1.CloudRelease{},
			&appsv1.Deployment{},
		).
		WithObjects(objects...).
		Build()

	return &CloudReleaseReconciler{
		Client: k8sClient,
		Scheme: testScheme,
	}, k8sClient
}

func unitTestCloudRelease() *deliveryv1alpha1.CloudRelease {
	return &deliveryv1alpha1.CloudRelease{
		TypeMeta: metav1.TypeMeta{
			APIVersion: deliveryv1alpha1.GroupVersion.String(),
			Kind:       "CloudRelease",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "demo",
			Namespace:  "default",
			UID:        types.UID("11111111-2222-3333-4444-555555555555"),
			Generation: 1,
		},
		Spec: deliveryv1alpha1.CloudReleaseSpec{
			Image:    "nginx:1.27-alpine",
			Replicas: 2,
			Port:     80,
		},
	}
}
