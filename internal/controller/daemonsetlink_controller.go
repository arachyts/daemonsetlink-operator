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
	"fmt"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorsv1alpha1 "github.com/arachyts/daemonsetlink-operator/api/v1alpha1"
)

var log = logf.Log.WithName("controller_daemonsetlink")

const OriginalNodeSelectorAnnotation = "operators.artnetlab.tech/original-nodeselector"

// DaemonSetLinkReconciler reconciles a DaemonSetLink object
type DaemonSetLinkReconciler struct {
	client.Client
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=operators.artnetlab.tech,resources=daemonsetlinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operators.artnetlab.tech,resources=daemonsetlinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operators.artnetlab.tech,resources=daemonsetlinks/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;patch

func (r *DaemonSetLinkReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling DaemonSetLink")

	// Fetch the DaemonSetLink instance
	dsLink := &operatorsv1alpha1.DaemonSetLink{}
	err := r.Get(ctx, req.NamespacedName, dsLink)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("DaemonSetLink resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get DaemonSetLink")
		return ctrl.Result{}, err
	}

	// Defer status update to ensure it happens even on early returns (like source not found)
	// Use a copy to avoid modifying the original object before patching status
	statusPatchBase := dsLink.DeepCopy()
	defer func() {
		// Only patch status if there are changes
		if !reflect.DeepEqual(statusPatchBase.Status, dsLink.Status) {
			if err := r.Status().Patch(ctx, dsLink, client.MergeFrom(statusPatchBase)); err != nil {
				if errors.IsConflict(err) {
					log.Info("Conflict during status update, retrying reconciliation.")
					// Don't return the error directly, let the main reconcile loop handle requeue if needed
				} else {
					log.Error(err, "Failed to update DaemonSetLink status")
				}
			}
		}
	}()

	// Update observed generation in status
	if dsLink.Status.ObservedGeneration != dsLink.Generation {
		dsLink.Status.ObservedGeneration = dsLink.Generation
	}

	// Get the configuration
	sourceRef := dsLink.Spec.SourceRef
	targetRef := dsLink.Spec.TargetDaemonSetRef
	disabledNodeSelector := dsLink.Spec.DisabledNodeSelector

	var sourceReplicas int32 = 0

	switch sourceRef.Kind {
	case "Deployment":
		deployment := &appsv1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{Name: sourceRef.Name, Namespace: sourceRef.Namespace}, deployment)
		if err != nil {
			if errors.IsNotFound(err) {
				// Source not found, log, update status, don't scale DS
				errMsg := fmt.Sprintf("Source Deployment %s/%s not found", sourceRef.Namespace, sourceRef.Name)
				log.Info(errMsg)
				updateStatusFields(dsLink, dsLink.Status.Linked, errMsg+", no action taken on target.") // Keep current linked state
				return ctrl.Result{}, nil                                                               // Stop reconciliation, no error
			}
			// Other error getting the Deployment
			errMsg := fmt.Sprintf("Error getting source Deployment %s/%s", sourceRef.Namespace, sourceRef.Name)
			log.Error(err, errMsg)
			updateStatusFields(dsLink, dsLink.Status.Linked, errMsg)
			return ctrl.Result{}, err // Requeue on transient error
		}
		if deployment.Spec.Replicas != nil {
			sourceReplicas = *deployment.Spec.Replicas
		}
	case "StatefulSet":
		statefulSet := &appsv1.StatefulSet{}
		err := r.Get(ctx, types.NamespacedName{Name: sourceRef.Name, Namespace: sourceRef.Namespace}, statefulSet)
		if err != nil {
			if errors.IsNotFound(err) {
				// Source not found, log, update status, don't scale DS
				errMsg := fmt.Sprintf("Source StatefulSet %s/%s not found", sourceRef.Namespace, sourceRef.Name)
				log.Info(errMsg)
				updateStatusFields(dsLink, dsLink.Status.Linked, errMsg+", no action taken on target.") // Keep current linked state
				return ctrl.Result{}, nil                                                               // Stop reconciliation, no error
			}
			// Other error getting the StatefulSet
			errMsg := fmt.Sprintf("Error getting source StatefulSet %s/%s", sourceRef.Namespace, sourceRef.Name)
			log.Error(err, errMsg)
			updateStatusFields(dsLink, dsLink.Status.Linked, errMsg)
			return ctrl.Result{}, err // Requeue on transient error
		}
		if statefulSet.Spec.Replicas != nil {
			sourceReplicas = *statefulSet.Spec.Replicas
		}
	default:
		err := fmt.Errorf("unsupported source kind: %s", sourceRef.Kind)
		log.Error(err, "Invalid source Kind specified in DaemonSetLink spec")
		updateStatusFields(dsLink, false, err.Error())
		return ctrl.Result{}, nil // Don't requeue for spec error
	}

	log.Info("Source found", "Source.Namespace", sourceRef.Namespace, "Source.Name", sourceRef.Name, "Source.Kind", sourceRef.Kind)

	// Get the target daemonset
	daemonSet := &appsv1.DaemonSet{}
	if err := r.Get(ctx, types.NamespacedName{Name: targetRef.Name, Namespace: targetRef.Namespace}, daemonSet); err != nil {
		if errors.IsNotFound(err) {
			errMsg := fmt.Sprintf("Target DaemonSet '%s/%s' not found", targetRef.Namespace, targetRef.Name)
			log.Error(nil, errMsg) // Use nil error for logging warning/info level
			updateStatusFields(dsLink, false, errMsg)
			return ctrl.Result{}, nil
		}
		errMsg := fmt.Sprintf("Error getting target DaemonSet %s/%s", targetRef.Namespace, targetRef.Name)
		log.Error(err, errMsg)
		updateStatusFields(dsLink, dsLink.Status.Linked, errMsg) // Keep old linked status on error
		return ctrl.Result{}, err                                // Requeue on transient error
	}

	// Determine Desired State
	currentNodeSelector := daemonSet.Spec.Template.Spec.NodeSelector
	currentAnnotations := daemonSet.Annotations
	if currentAnnotations == nil {
		currentAnnotations = make(map[string]string)
	}

	// Create a patch base using a deep copy BEFORE making modifications
	patchBase := client.MergeFrom(daemonSet.DeepCopy())
	modifiedDS := daemonSet // Work on the original DS object for patching

	needsPatch := false
	desiredLinkedState := dsLink.Status.Linked // Default to current status

	if sourceReplicas > 0 {
		// --- Source is Active: Enable DaemonSet ---
		desiredLinkedState = true
		// Check if DS is currently disabled by us
		if reflect.DeepEqual(currentNodeSelector, disabledNodeSelector) {
			log.Info("Source is active, enabling target DaemonSet", "DaemonSet", targetRef.Name)

			// Try to restore original selector from annotation
			originalSelectorJSON, exists := currentAnnotations[OriginalNodeSelectorAnnotation]
			restoredSelector := make(map[string]string) // Default to empty map if no annotation/error

			if exists {
				log.Info("Found original nodeSelector annotation, attempting restore.")
				if err := json.Unmarshal([]byte(originalSelectorJSON), &restoredSelector); err != nil {
					log.Error(err, "Failed to unmarshal original nodeSelector from annotation, defaulting to nil selector", "AnnotationValue", originalSelectorJSON)
					// Don't fail, just proceed with nil/empty selector which still enables it
					restoredSelector = nil
				} else {
					log.Info("Successfully restored nodeSelector from annotation", "RestoredSelector", restoredSelector)
				}
				// Remove the annotation now that we've restored it
				delete(modifiedDS.Annotations, OriginalNodeSelectorAnnotation)
			} else {
				log.Info("No original nodeSelector annotation found, enabling DaemonSet with nil nodeSelector.")
				restoredSelector = nil // Ensure it's nil if no annotation
			}

			modifiedDS.Spec.Template.Spec.NodeSelector = restoredSelector
			needsPatch = true

		} else {
			log.Info("Source is active and target DaemonSet already appears enabled", "DaemonSet", targetRef.Name)
			// Ensure annotation is removed if DS was somehow enabled manually while annotation existed
			if _, exists := currentAnnotations[OriginalNodeSelectorAnnotation]; exists {
				log.Info("Removing stale original nodeSelector annotation from enabled DaemonSet.")
				delete(modifiedDS.Annotations, OriginalNodeSelectorAnnotation)
				needsPatch = true
			}
		}
	} else {
		// --- Source is Inactive: Disable DaemonSet ---
		desiredLinkedState = false
		// Check if DS is currently enabled (or has a different selector)
		if !reflect.DeepEqual(currentNodeSelector, disabledNodeSelector) {
			log.Info("Source is inactive, disabling target DaemonSet", "DaemonSet", targetRef.Name)

			// Store the current selector in the annotation BEFORE overwriting
			currentSelectorJSON, err := json.Marshal(currentNodeSelector)
			if err != nil {
				// This should realistically never happen with map[string]string
				log.Error(err, "Failed to marshal current nodeSelector to JSON, cannot store original selector.")
				// Proceed to disable without storing? Or return error? Let's proceed but log heavily.
				updateStatusFields(dsLink, desiredLinkedState, "Error storing original nodeSelector, but proceeding to disable.")
				// Do not set the annotation if marshalling failed
			} else {
				log.Info("Storing current nodeSelector in annotation", "Selector", currentNodeSelector)
				if modifiedDS.Annotations == nil { // Ensure map exists before writing
					modifiedDS.Annotations = make(map[string]string)
				}
				modifiedDS.Annotations[OriginalNodeSelectorAnnotation] = string(currentSelectorJSON)
			}

			// Apply the disabled selector
			modifiedDS.Spec.Template.Spec.NodeSelector = disabledNodeSelector
			needsPatch = true

		} else {
			log.Info("Source is inactive and target DaemonSet already appears disabled", "DaemonSet", targetRef.Name)
		}
	}

	// Apply Patch if necessary
	if needsPatch {
		log.Info("Applying patch to target DaemonSet", "DaemonSet", targetRef.Name)
		if err := r.Patch(ctx, modifiedDS, patchBase); err != nil { // Use modifiedDS and the patchBase
			log.Error(err, "Failed to patch target DaemonSet", "DaemonSet.Namespace", targetRef.Namespace, "DaemonSet.Name", targetRef.Name)
			updateStatusFields(dsLink, dsLink.Status.Linked, fmt.Sprintf("Error patching target DaemonSet: %v", err)) // Keep old linked status on patch error
			return ctrl.Result{}, err                                                                                 // Requeue on transient error
		}
		log.Info("Successfully patched target DaemonSet", "DaemonSet", targetRef.Name)
		r.Recorder.Eventf(dsLink, "Normal", "DaemonSetPatched", "Patched DaemonSet %s/%s based on source %s/%s", targetRef.Namespace, targetRef.Name, sourceRef.Namespace, sourceRef.Name)
	}

	// Update DaemonSetLink status
	finalMsg := "Link synchronized successfully"
	if !desiredLinkedState {
		finalMsg = "Link synchronized, DaemonSet disabled due to inactive source"
	}
	updateStatusFields(dsLink, desiredLinkedState, finalMsg)

	log.Info("Reconciliation finished successfully")
	return ctrl.Result{}, nil
}

// Modifies the status fields in the passed DaemonSetLink object.
// It does NOT patch the object; the caller is responsible for patching.
func updateStatusFields(dsLink *operatorsv1alpha1.DaemonSetLink, linked bool, message string) {
	dsLink.Status.Linked = linked
	dsLink.Status.Message = message
}

// SetupWithManager sets up the controller with the Manager.
func (r *DaemonSetLinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor("daemonsetlink-controller")
	}

	// mapFn maps Deployment/StatefulSet events to DaemonSetLink reconcile requests
	mapFn := func(ctx context.Context, obj client.Object) []reconcile.Request {
		reqLog := logf.FromContext(ctx) // Use context logger
		linkList := &operatorsv1alpha1.DaemonSetLinkList{}

		// List all DaemonSetLinks - Consider using an index for scalability if many links exist
		// If CRs are namespaced, use client.MatchingFields or filter manually based on obj.GetNamespace() if needed
		if err := r.List(ctx, linkList /*, client.InNamespace(obj.GetNamespace())*/); err != nil {
			reqLog.Error(err, "Failed to list DaemonSetLinks to map source object change")
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, 0, len(linkList.Items)) // Pre-allocate slice
		sourceKind := obj.GetObjectKind().GroupVersionKind().Kind
		sourceName := obj.GetName()
		sourceNamespace := obj.GetNamespace()

		reqLog.V(1).Info("Checking DaemonSetLinks for source object change", "SourceKind", sourceKind, "SourceName", sourceName, "SourceNamespace", sourceNamespace)

		for _, item := range linkList.Items {
			if item.Spec.SourceRef.Kind == sourceKind &&
				item.Spec.SourceRef.Name == sourceName &&
				item.Spec.SourceRef.Namespace == sourceNamespace {

				reqLog.Info("Detected change in linked source object, queueing DaemonSetLink reconcile",
					"SourceKind", sourceKind, "SourceName", sourceName, "SourceNamespace", sourceNamespace,
					"DaemonSetLink", types.NamespacedName{Name: item.Name, Namespace: item.Namespace})

				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      item.Name,
						Namespace: item.Namespace,
					},
				})
			}
		}
		return requests
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorsv1alpha1.DaemonSetLink{}).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(mapFn),
		).
		Watches(
			&appsv1.StatefulSet{},
			handler.EnqueueRequestsFromMapFunc(mapFn),
		).
		Complete(r)
}
