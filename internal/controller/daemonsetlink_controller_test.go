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
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorsv1alpha1 "github.com/arachyts/daemonsetlink-operator/api/v1alpha1"
)

var _ = Describe("DaemonSetLink Controller", func() {
	const (
		namespace           = "default"
		daemonSetLinkName   = "test-daemonsetlink"
		deploymentName      = "test-deployment"
		statefulSetName     = "test-statefulset"
		daemonSetName       = "test-daemonset"
		timeout             = time.Second * 10
		interval            = time.Millisecond * 250
		nonExistentSelector = "nonexistent.daemonsetlink.operator/disable"
	)

	var (
		ctx                    context.Context
		daemonSetLinkLookupKey types.NamespacedName
		reconciler             *DaemonSetLinkReconciler
		fakeRecorder           *record.FakeRecorder
	)

	// Helper function to clean up test resources
	cleanupTestResources := func(ctx context.Context) {
		// Delete DaemonSetLink if it exists
		daemonSetLink := &operatorsv1alpha1.DaemonSetLink{}
		err := k8sClient.Get(ctx, daemonSetLinkLookupKey, daemonSetLink)
		if err == nil {
			Expect(k8sClient.Delete(ctx, daemonSetLink)).To(Succeed())
		}

		// Delete Deployment if it exists
		deployment := &appsv1.Deployment{}
		err = k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, deployment)
		if err == nil {
			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())
		}

		// Delete StatefulSet if it exists
		statefulSet := &appsv1.StatefulSet{}
		err = k8sClient.Get(ctx, types.NamespacedName{Name: statefulSetName, Namespace: namespace}, statefulSet)
		if err == nil {
			Expect(k8sClient.Delete(ctx, statefulSet)).To(Succeed())
		}

		// Delete DaemonSet if it exists
		daemonSet := &appsv1.DaemonSet{}
		err = k8sClient.Get(ctx, types.NamespacedName{Name: daemonSetName, Namespace: namespace}, daemonSet)
		if err == nil {
			Expect(k8sClient.Delete(ctx, daemonSet)).To(Succeed())
		}
	}

	BeforeEach(func() {
		ctx = context.Background()
		daemonSetLinkLookupKey = types.NamespacedName{Name: daemonSetLinkName, Namespace: namespace}

		// Create a fake event recorder
		fakeRecorder = record.NewFakeRecorder(10)

		// Create the reconciler with the test client and fake recorder
		reconciler = &DaemonSetLinkReconciler{
			Client:   k8sClient,
			Recorder: fakeRecorder,
		}
	})

	AfterEach(func() {
		// Clean up resources after each test
		cleanupTestResources(ctx)
	})

	// Helper function to create a DaemonSetLink with Deployment as source
	createDaemonSetLinkWithDeployment := func() *operatorsv1alpha1.DaemonSetLink {
		daemonSetLink := &operatorsv1alpha1.DaemonSetLink{
			ObjectMeta: metav1.ObjectMeta{
				Name:      daemonSetLinkName,
				Namespace: namespace,
			},
			Spec: operatorsv1alpha1.DaemonSetLinkSpec{
				SourceRef: operatorsv1alpha1.SourceRef{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       deploymentName,
					Namespace:  namespace,
				},
				TargetDaemonSetRef: operatorsv1alpha1.TargetDaemonSetRef{
					Name:      daemonSetName,
					Namespace: namespace,
				},
				DisabledNodeSelector: map[string]string{
					nonExistentSelector: "true",
				},
			},
		}
		Expect(k8sClient.Create(ctx, daemonSetLink)).To(Succeed())
		return daemonSetLink
	}

	// Helper function to create a DaemonSetLink with StatefulSet as source
	createDaemonSetLinkWithStatefulSet := func() *operatorsv1alpha1.DaemonSetLink {
		daemonSetLink := &operatorsv1alpha1.DaemonSetLink{
			ObjectMeta: metav1.ObjectMeta{
				Name:      daemonSetLinkName,
				Namespace: namespace,
			},
			Spec: operatorsv1alpha1.DaemonSetLinkSpec{
				SourceRef: operatorsv1alpha1.SourceRef{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       statefulSetName,
					Namespace:  namespace,
				},
				TargetDaemonSetRef: operatorsv1alpha1.TargetDaemonSetRef{
					Name:      daemonSetName,
					Namespace: namespace,
				},
				DisabledNodeSelector: map[string]string{
					nonExistentSelector: "true",
				},
			},
		}
		Expect(k8sClient.Create(ctx, daemonSetLink)).To(Succeed())
		return daemonSetLink
	}

	// Helper function to create a Deployment with specified replicas
	createDeployment := func(replicas int32) *appsv1.Deployment {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, deployment)).To(Succeed())
		return deployment
	}

	// Helper function to create a StatefulSet with specified replicas
	createStatefulSet := func(replicas int32) *appsv1.StatefulSet {
		statefulSet := &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      statefulSetName,
				Namespace: namespace,
			},
			Spec: appsv1.StatefulSetSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test",
					},
				},
				ServiceName: "test-service",
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, statefulSet)).To(Succeed())
		return statefulSet
	}

	// Helper function to create a DaemonSet with specified node selector
	createDaemonSet := func(nodeSelector map[string]string) *appsv1.DaemonSet {
		daemonSet := &appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      daemonSetName,
				Namespace: namespace,
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "test-ds",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "test-ds",
						},
					},
					Spec: corev1.PodSpec{
						NodeSelector: nodeSelector,
						Containers: []corev1.Container{
							{
								Name:  "test-container",
								Image: "nginx:latest",
							},
						},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, daemonSet)).To(Succeed())
		return daemonSet
	}

	// Helper function to reconcile the DaemonSetLink
	reconcileDaemonSetLink := func() {
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: daemonSetLinkLookupKey,
		})
		Expect(err).NotTo(HaveOccurred())
	}

	// Helper function to get the current DaemonSet
	getDaemonSet := func() *appsv1.DaemonSet {
		daemonSet := &appsv1.DaemonSet{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: daemonSetName, Namespace: namespace}, daemonSet)
		Expect(err).NotTo(HaveOccurred())
		return daemonSet
	}

	// Helper function to get the current DaemonSetLink
	getDaemonSetLink := func() *operatorsv1alpha1.DaemonSetLink {
		daemonSetLink := &operatorsv1alpha1.DaemonSetLink{}
		err := k8sClient.Get(ctx, daemonSetLinkLookupKey, daemonSetLink)
		Expect(err).NotTo(HaveOccurred())
		return daemonSetLink
	}

	// Helper function to update Deployment replicas
	updateDeploymentReplicas := func(replicas int32) {
		deployment := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, deployment)
		Expect(err).NotTo(HaveOccurred())

		deployment.Spec.Replicas = &replicas
		Expect(k8sClient.Update(ctx, deployment)).To(Succeed())
	}

	Context("When using a Deployment as source", func() {
		It("should disable the DaemonSet when Deployment has 0 replicas", func() {
			// Create a Deployment with 0 replicas
			createDeployment(0)

			// Create a DaemonSet with empty node selector
			originalNodeSelector := map[string]string{"role": "worker"}
			createDaemonSet(originalNodeSelector)

			// Create the DaemonSetLink
			createDaemonSetLinkWithDeployment()

			// Reconcile
			reconcileDaemonSetLink()

			// Check that the DaemonSet has been disabled with the non-existent node selector
			daemonSet := getDaemonSet()
			Expect(daemonSet.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue(nonExistentSelector, "true"))

			// Check that the original node selector is stored in the annotation
			originalSelectorJSON, exists := daemonSet.Annotations[OriginalNodeSelectorAnnotation]
			Expect(exists).To(BeTrue())

			var storedSelector map[string]string
			err := json.Unmarshal([]byte(originalSelectorJSON), &storedSelector)
			Expect(err).NotTo(HaveOccurred())
			Expect(storedSelector).To(Equal(originalNodeSelector))

			// Check the DaemonSetLink status
			daemonSetLink := getDaemonSetLink()
			Expect(daemonSetLink.Status.Linked).To(BeFalse())
			Expect(daemonSetLink.Status.Message).To(ContainSubstring("DaemonSet disabled"))
		})

		It("should enable the DaemonSet when Deployment has > 0 replicas", func() {
			// Create a Deployment with 1 replica
			createDeployment(1)

			// Create a DaemonSet with the disabled node selector
			disabledNodeSelector := map[string]string{nonExistentSelector: "true"}
			createDaemonSet(disabledNodeSelector)

			// Create the DaemonSetLink
			createDaemonSetLinkWithDeployment()

			// Reconcile
			reconcileDaemonSetLink()

			// Check that the DaemonSet has been enabled (empty node selector)
			daemonSet := getDaemonSet()
			Expect(daemonSet.Spec.Template.Spec.NodeSelector).To(BeEmpty())

			// Check that the annotation has been removed
			_, exists := daemonSet.Annotations[OriginalNodeSelectorAnnotation]
			Expect(exists).To(BeFalse())

			// Check the DaemonSetLink status
			daemonSetLink := getDaemonSetLink()
			Expect(daemonSetLink.Status.Linked).To(BeTrue())
			Expect(daemonSetLink.Status.Message).To(ContainSubstring("synchronized successfully"))
		})

		It("should restore the original node selector when enabling the DaemonSet", func() {
			// Create a Deployment with 0 replicas initially
			createDeployment(0)

			// Create a DaemonSet with a specific node selector
			originalNodeSelector := map[string]string{"role": "worker", "zone": "us-east-1"}
			createDaemonSet(originalNodeSelector)

			// Create the DaemonSetLink
			createDaemonSetLinkWithDeployment()

			// Reconcile to disable the DaemonSet
			reconcileDaemonSetLink()

			// Verify the DaemonSet is disabled
			daemonSet := getDaemonSet()
			Expect(daemonSet.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue(nonExistentSelector, "true"))

			// Update the Deployment to have replicas
			updateDeploymentReplicas(2)

			// Reconcile again to enable the DaemonSet
			reconcileDaemonSetLink()

			// Check that the original node selector has been restored
			daemonSet = getDaemonSet()
			Expect(daemonSet.Spec.Template.Spec.NodeSelector).To(Equal(originalNodeSelector))

			// Check the DaemonSetLink status
			daemonSetLink := getDaemonSetLink()
			Expect(daemonSetLink.Status.Linked).To(BeTrue())
		})

		It("should handle missing Deployment gracefully", func() {
			// Create a DaemonSet
			createDaemonSet(nil)

			// Create the DaemonSetLink without creating the Deployment
			createDaemonSetLinkWithDeployment()

			// Reconcile
			reconcileDaemonSetLink()

			// Check that the DaemonSet was not modified
			daemonSet := getDaemonSet()
			Expect(daemonSet.Spec.Template.Spec.NodeSelector).To(BeEmpty())

			// Check the DaemonSetLink status reflects the missing Deployment
			daemonSetLink := getDaemonSetLink()
			Expect(daemonSetLink.Status.Message).To(ContainSubstring("not found"))
		})
	})

	Context("When using a StatefulSet as source", func() {
		It("should disable the DaemonSet when StatefulSet has 0 replicas", func() {
			// Create a StatefulSet with 0 replicas
			createStatefulSet(0)

			// Create a DaemonSet with empty node selector
			originalNodeSelector := map[string]string{"role": "worker"}
			createDaemonSet(originalNodeSelector)

			// Create the DaemonSetLink
			createDaemonSetLinkWithStatefulSet()

			// Reconcile
			reconcileDaemonSetLink()

			// Check that the DaemonSet has been disabled with the non-existent node selector
			daemonSet := getDaemonSet()
			Expect(daemonSet.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue(nonExistentSelector, "true"))

			// Check that the original node selector is stored in the annotation
			originalSelectorJSON, exists := daemonSet.Annotations[OriginalNodeSelectorAnnotation]
			Expect(exists).To(BeTrue())

			var storedSelector map[string]string
			err := json.Unmarshal([]byte(originalSelectorJSON), &storedSelector)
			Expect(err).NotTo(HaveOccurred())
			Expect(storedSelector).To(Equal(originalNodeSelector))

			// Check the DaemonSetLink status
			daemonSetLink := getDaemonSetLink()
			Expect(daemonSetLink.Status.Linked).To(BeFalse())
			Expect(daemonSetLink.Status.Message).To(ContainSubstring("DaemonSet disabled"))
		})

		It("should enable the DaemonSet when StatefulSet has > 0 replicas", func() {
			// Create a StatefulSet with 1 replica
			createStatefulSet(1)

			// Create a DaemonSet with the disabled node selector
			disabledNodeSelector := map[string]string{nonExistentSelector: "true"}
			createDaemonSet(disabledNodeSelector)

			// Create the DaemonSetLink
			createDaemonSetLinkWithStatefulSet()

			// Reconcile
			reconcileDaemonSetLink()

			// Check that the DaemonSet has been enabled (empty node selector)
			daemonSet := getDaemonSet()
			Expect(daemonSet.Spec.Template.Spec.NodeSelector).To(BeEmpty())

			// Check that the annotation has been removed
			_, exists := daemonSet.Annotations[OriginalNodeSelectorAnnotation]
			Expect(exists).To(BeFalse())

			// Check the DaemonSetLink status
			daemonSetLink := getDaemonSetLink()
			Expect(daemonSetLink.Status.Linked).To(BeTrue())
			Expect(daemonSetLink.Status.Message).To(ContainSubstring("synchronized successfully"))
		})

		It("should handle missing StatefulSet gracefully", func() {
			// Create a DaemonSet
			createDaemonSet(nil)

			// Create the DaemonSetLink without creating the StatefulSet
			createDaemonSetLinkWithStatefulSet()

			// Reconcile
			reconcileDaemonSetLink()

			// Check that the DaemonSet was not modified
			daemonSet := getDaemonSet()
			Expect(daemonSet.Spec.Template.Spec.NodeSelector).To(BeEmpty())

			// Check the DaemonSetLink status reflects the missing StatefulSet
			daemonSetLink := getDaemonSetLink()
			Expect(daemonSetLink.Status.Message).To(ContainSubstring("not found"))
		})
	})

	Context("When handling edge cases", func() {
		It("should handle missing target DaemonSet gracefully", func() {
			// Create a Deployment with 1 replica
			createDeployment(1)

			// Create the DaemonSetLink without creating the DaemonSet
			createDaemonSetLinkWithDeployment()

			// Reconcile
			reconcileDaemonSetLink()

			// Check the DaemonSetLink status reflects the missing DaemonSet
			daemonSetLink := getDaemonSetLink()
			Expect(daemonSetLink.Status.Linked).To(BeFalse())
			Expect(daemonSetLink.Status.Message).To(ContainSubstring("not found"))
		})
	})
})
