/*
Copyright 2025.

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	chaosv1alpha1 "kubechaos-operator/api/v1alpha1"
)

var _ = Describe("ChaosExperiment Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default" // Using default for simplicity in test
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		chaosexperiment := &chaosv1alpha1.ChaosExperiment{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ChaosExperiment")
			// Ensure the resource is not already present from a failed previous test
			_ = k8sClient.Get(ctx, typeNamespacedName, chaosexperiment)
			if chaosexperiment.Name != "" {
				Expect(k8sClient.Delete(ctx, chaosexperiment)).To(Succeed())
				// Wait for deletion
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, chaosexperiment))
				}, time.Second*5, time.Millisecond*500).Should(BeTrue())
			}

			resource := &chaosv1alpha1.ChaosExperiment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: chaosv1alpha1.ChaosExperimentSpec{
					Target: chaosv1alpha1.ExperimentTarget{
						Namespace: "test-namespace", // A valid namespace, though not created in envtest
						LabelSelector: map[string]string{
							"app": "test-app", // A valid label selector
						},
					},
					Attack: chaosv1alpha1.ExperimentAttack{
						Type: chaosv1alpha1.PodKillAttack, // The only supported attack type
					},
					Duration: &metav1.Duration{Duration: 30 * time.Second}, // Optional: 30 seconds
					Mode:     chaosv1alpha1.OneShotMode,                    // Optional: "one-shot"
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			By("Cleanup the specific resource instance ChaosExperiment")
			resource := &chaosv1alpha1.ChaosExperiment{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil { // Only delete if it exists
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				// Wait for deletion
				Eventually(func() bool {
					return errors.IsNotFound(k8sClient.Get(ctx, typeNamespacedName, resource))
				}, time.Second*5, time.Millisecond*500).Should(BeTrue())
			}
		})

		It("should successfully reconcile the resource and set its phase to Pending", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ChaosExperimentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				// Recorder for events
				Recorder: record.NewFakeRecorder(100),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Assert that the ChaosExperiment's status changes to Pending
			Eventually(func() string {
				_ = k8sClient.Get(ctx, typeNamespacedName, chaosexperiment)
				return string(chaosexperiment.Status.Phase)
			}, time.Second*5, time.Millisecond*500).Should(Equal(string(chaosv1alpha1.ExperimentPending)))
		})
	})
})
