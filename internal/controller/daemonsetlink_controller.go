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
	"github.com/go-logr/logr"
)

// Name for node selector annotation - used to save original node selectors
const OriginalNodeSelectorAnnotation = "operators.artnetlab.tech/original-nodeselector"

// sourceReplicasResult holds the result of getting source replicas
type sourceReplicasResult struct {
	replicas int32
	err      error
	errMsg   string
}

// DaemonSetLinkReconciler reconciles a DaemonSetLink object
type DaemonSetLinkReconciler struct {
	client.Client
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=operators.artnetlab.tech,resources=daemonsetlinks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operators.artnetlab.tech,resources=daemonsetlinks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=operators.artnetlab.tech,resources=daemonsetlinks/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// getSourceReplicas retrieves the replicas from a Deployment or StatefulSet
// Returns the replica count, error, and error message
func (r *DaemonSetLinkReconciler) getSourceReplicas(ctx context.Context, sourceRef operatorsv1alpha1.SourceRef) sourceReplicasResult {
	result := sourceReplicasResult{
		replicas: 0, // Default to 0 replicas
	}

	switch sourceRef.Kind {
	case "Deployment":
		deployment := &appsv1.Deployment{}
		err := r.Get(ctx, types.NamespacedName{Name: sourceRef.Name, Namespace: sourceRef.Namespace}, deployment)
		if err != nil {
			if errors.IsNotFound(err) {
				result.errMsg = fmt.Sprintf("Source Deployment %s/%s not found", sourceRef.Namespace, sourceRef.Name)
				result.err = err
				return result
			}
			result.err = err
			result.errMsg = fmt.Sprintf("Error getting source Deployment %s/%s", sourceRef.Namespace, sourceRef.Name)
			return result
		}
		if deployment.Spec.Replicas != nil {
			result.replicas = *deployment.Spec.Replicas
		} else {
			result.errMsg = fmt.Sprintf("Source Deployment %s/%s has no replicas set", sourceRef.Namespace, sourceRef.Name)
			result.err = fmt.Errorf("source deployment has no replicas set")
			return result
		}
	case "StatefulSet":
		statefulSet := &appsv1.StatefulSet{}
		err := r.Get(ctx, types.NamespacedName{Name: sourceRef.Name, Namespace: sourceRef.Namespace}, statefulSet)
		if err != nil {
			if errors.IsNotFound(err) {
				result.errMsg = fmt.Sprintf("Source StatefulSet %s/%s not found", sourceRef.Namespace, sourceRef.Name)
				result.err = err
				return result
			}
			result.err = err
			result.errMsg = fmt.Sprintf("Error getting source StatefulSet %s/%s", sourceRef.Namespace, sourceRef.Name)
			return result
		}
		if statefulSet.Spec.Replicas != nil {
			result.replicas = *statefulSet.Spec.Replicas
		} else {
			result.errMsg = fmt.Sprintf("Source StatefulSet %s/%s has no replicas set", sourceRef.Namespace, sourceRef.Name)
			result.err = fmt.Errorf("source statefulset has no replicas set")
			return result
		}
	default:
		result.err = fmt.Errorf("unsupported source kind: %s", sourceRef.Kind)
		result.errMsg = result.err.Error()
	}

	return result
}

// enableDaemonSet restores the original node selector from annotation
// Returns whether a patch is needed
func enableDaemonSet(daemonSet *appsv1.DaemonSet, disabledNodeSelector map[string]string, logger logr.Logger) (bool, map[string]string) {
	currentNodeSelector := daemonSet.Spec.Template.Spec.NodeSelector
	currentAnnotations := daemonSet.Annotations
	if currentAnnotations == nil {
		currentAnnotations = make(map[string]string)
	}

	// Check if DS is currently disabled by us
	if !reflect.DeepEqual(currentNodeSelector, disabledNodeSelector) {
		// If not disabled with our selector, just check for stale annotation
		if _, exists := currentAnnotations[OriginalNodeSelectorAnnotation]; exists {
			logger.Info("Removing stale original nodeSelector annotation from enabled DaemonSet.")
			delete(daemonSet.Annotations, OriginalNodeSelectorAnnotation)
			return true, currentNodeSelector // Need patch to remove annotation
		}
		return false, currentNodeSelector // No changes needed
	}

	// DS is disabled, need to enable it
	logger.Info("Enabling target DaemonSet")

	// Try to restore original selector from annotation
	originalSelectorJSON, exists := currentAnnotations[OriginalNodeSelectorAnnotation]
	restoredSelector := make(map[string]string) // Default to empty map if no annotation/error

	if exists {
		logger.Info("Found original nodeSelector annotation, attempting restore.")
		if err := json.Unmarshal([]byte(originalSelectorJSON), &restoredSelector); err != nil {
			logger.Error(err, "Failed to unmarshal original nodeSelector from annotation, defaulting to nil selector", "AnnotationValue", originalSelectorJSON)
			// Don't fail, just proceed with nil/empty selector which still enables it
			restoredSelector = nil
		} else {
			logger.Info("Successfully restored nodeSelector from annotation", "RestoredSelector", restoredSelector)
		}
		// Remove the annotation now that we've restored it
		delete(daemonSet.Annotations, OriginalNodeSelectorAnnotation)
	} else {
		logger.Info("No original nodeSelector annotation found, enabling DaemonSet with nil nodeSelector.")
		restoredSelector = nil // Ensure it's nil if no annotation
	}

	daemonSet.Spec.Template.Spec.NodeSelector = restoredSelector
	return true, restoredSelector // Need patch to restore selector
}

// disableDaemonSet applies the disabled node selector and stores the original
// Returns whether a patch is needed and any error message
func disableDaemonSet(daemonSet *appsv1.DaemonSet, disabledNodeSelector map[string]string, logger logr.Logger) (bool, string) {
	currentNodeSelector := daemonSet.Spec.Template.Spec.NodeSelector

	// Check if DS is already disabled with our selector
	if reflect.DeepEqual(currentNodeSelector, disabledNodeSelector) {
		logger.Info("Target DaemonSet already appears disabled")
		return false, "" // No changes needed
	}

	// DS needs to be disabled
	logger.Info("Disabling target DaemonSet")

	// Store the current selector in the annotation BEFORE overwriting
	currentSelectorJSON, err := json.Marshal(currentNodeSelector)
	if err != nil {
		// This should realistically never happen with map[string]string
		logger.Error(err, "Failed to marshal current nodeSelector to JSON, cannot store original selector.")
		errMsg := "Error storing original nodeSelector, but proceeding to disable."
		// Proceed to disable without storing
		daemonSet.Spec.Template.Spec.NodeSelector = disabledNodeSelector
		return true, errMsg
	}

	logger.Info("Storing current nodeSelector in annotation", "Selector", currentNodeSelector)
	if daemonSet.Annotations == nil { // Ensure map exists before writing
		daemonSet.Annotations = make(map[string]string)
	}
	daemonSet.Annotations[OriginalNodeSelectorAnnotation] = string(currentSelectorJSON)

	// Apply the disabled selector
	daemonSet.Spec.Template.Spec.NodeSelector = disabledNodeSelector
	return true, "" // Need patch to disable
}

// Reconcile handles the reconciliation loop for DaemonSetLink resources
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

	// Get source replicas
	sourceResult := r.getSourceReplicas(ctx, sourceRef)
	if sourceResult.err != nil {
		if errors.IsNotFound(sourceResult.err) {
			// Source not found
			log.Info(sourceResult.errMsg)
			updateStatusFields(dsLink, dsLink.Status.Linked, sourceResult.errMsg+", no action taken on target.")
			return ctrl.Result{}, nil // nil for NotFound, causing no requeue
		}
		// Other error getting the source
		log.Error(sourceResult.err, sourceResult.errMsg)
		updateStatusFields(dsLink, dsLink.Status.Linked, sourceResult.errMsg)
		return ctrl.Result{}, sourceResult.err // Requeue on transient error
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

	// Create a patch base using a deep copy before making modifications
	patchBase := client.MergeFrom(daemonSet.DeepCopy())
	needsPatch := false
	var desiredLinkedState bool
	var statusMsg string

	if sourceResult.replicas > 0 {
		// --- Source is Active: Enable DaemonSet ---
		desiredLinkedState = true
		var needsEnablePatch bool
		needsEnablePatch, _ = enableDaemonSet(daemonSet, disabledNodeSelector, log)
		needsPatch = needsEnablePatch
	} else {
		// --- Source is Inactive: Disable DaemonSet ---
		desiredLinkedState = false
		var needsDisablePatch bool
		var errMsg string
		needsDisablePatch, errMsg = disableDaemonSet(daemonSet, disabledNodeSelector, log)
		needsPatch = needsDisablePatch
		if errMsg != "" {
			statusMsg = errMsg
		}
	}

	// Apply Patch if necessary
	if needsPatch {
		log.Info("Applying patch to target DaemonSet", "DaemonSet", targetRef.Name)
		if err := r.Patch(ctx, daemonSet, patchBase); err != nil {
			log.Error(err, "Failed to patch target DaemonSet", "DaemonSet.Namespace", targetRef.Namespace, "DaemonSet.Name", targetRef.Name)
			updateStatusFields(dsLink, dsLink.Status.Linked, fmt.Sprintf("Error patching target DaemonSet: %v", err)) // Keep old linked status on patch error
			return ctrl.Result{}, err                                                                                 // Requeue on transient error
		}
		log.Info("Successfully patched target DaemonSet", "DaemonSet", targetRef.Name)
		r.Recorder.Eventf(dsLink, "Normal", "DaemonSetPatched", "Patched DaemonSet %s/%s based on source %s/%s", targetRef.Namespace, targetRef.Name, sourceRef.Namespace, sourceRef.Name)
	}

	// Update DaemonSetLink status
	if statusMsg == "" {
		if desiredLinkedState {
			statusMsg = "Link synchronized successfully"
		} else {
			statusMsg = "Link synchronized, DaemonSet disabled due to inactive source"
		}
	}
	updateStatusFields(dsLink, desiredLinkedState, statusMsg)

	log.Info("Reconciliation finished successfully")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DaemonSetLinkReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		r.Recorder = mgr.GetEventRecorderFor("daemonsetlink-controller")
	}

	// mapSourceToRequests maps source objects (Deployment/StatefulSet) to DaemonSetLink reconcile requests
	mapSourceToRequests := func(ctx context.Context, obj client.Object) []reconcile.Request {
		reqLog := logf.FromContext(ctx)
		linkList := &operatorsv1alpha1.DaemonSetLinkList{}

		// List all DaemonSetLinks
		if err := r.List(ctx, linkList); err != nil {
			reqLog.Error(err, "Failed to list DaemonSetLinks to map source object change")
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, 0, len(linkList.Items))
		sourceName := obj.GetName()
		sourceNamespace := obj.GetNamespace()
		sourceKind := getSourceKind(obj)

		reqLog.Info("Processing source object change",
			"Kind", sourceKind,
			"Name", sourceName,
			"Namespace", sourceNamespace)

		// Find matching DaemonSetLinks
		for _, item := range linkList.Items {
			if item.Spec.SourceRef.Name == sourceName &&
				item.Spec.SourceRef.Namespace == sourceNamespace &&
				item.Spec.SourceRef.Kind == sourceKind {

				reqLog.Info("Matched source object change to DaemonSetLink, queueing reconcile",
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
			handler.EnqueueRequestsFromMapFunc(mapSourceToRequests),
		).
		Watches(
			&appsv1.StatefulSet{},
			handler.EnqueueRequestsFromMapFunc(mapSourceToRequests),
		).
		Complete(r)
}

// getSourceKind determines the kind of a source object
func getSourceKind(obj client.Object) string {
	// Try to get kind from GroupVersionKind
	sourceKind := obj.GetObjectKind().GroupVersionKind().Kind

	// If GVK doesn't have Kind (common in tests), use reflection
	if sourceKind == "" {
		objType := reflect.TypeOf(obj)
		if objType.Kind() == reflect.Ptr {
			objType = objType.Elem()
		}
		sourceKind = objType.Name() // e.g., "Deployment", "StatefulSet"
	}

	return sourceKind
}

// Modifies the status fields in the passed DaemonSetLink object.
// It does NOT patch the object; the caller is responsible for patching.
func updateStatusFields(dsLink *operatorsv1alpha1.DaemonSetLink, linked bool, message string) {
	dsLink.Status.Linked = linked
	dsLink.Status.Message = message
}
