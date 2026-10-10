//go:build envtest

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

package envtest

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	deliveryv1alpha1 "github.com/senak1129/cloudrelease-operator/api/v1alpha1"
)

var _ = Describe("CloudRelease CRD", func() {
	const (
		namespace = "default"
		name      = "test-defaults"
	)

	AfterEach(func() {
		_ = k8sClient.Delete(context.Background(), &deliveryv1alpha1.CloudRelease{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		})
	})

	It("fills defaults when replicas and port are omitted", func(ctx SpecContext) {
		By("creating a CloudRelease with only image set")
		cr := &deliveryv1alpha1.CloudRelease{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: deliveryv1alpha1.CloudReleaseSpec{
				Image: "nginx:1.27-alpine",
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())

		By("reading back the object and checking defaults")
		got := &deliveryv1alpha1.CloudRelease{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, got)).To(Succeed())

		Expect(got.Spec.Replicas).To(Equal(int32(1)), "replicas should default to 1")
		Expect(got.Spec.Port).To(Equal(int32(8080)), "port should default to 8080")
	})

	It("rejects empty image", func(ctx SpecContext) {
		By("creating a CloudRelease with empty image")
		cr := &deliveryv1alpha1.CloudRelease{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec: deliveryv1alpha1.CloudReleaseSpec{
				Image: "",
			},
		}
		err := k8sClient.Create(ctx, cr)
		Expect(err).To(HaveOccurred(), "empty image should be rejected by API server")
	})
})
