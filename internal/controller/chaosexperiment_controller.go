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
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	chaosv1alpha1 "kubechaos-operator/api/v1alpha1"
)

// ChaosExperimentReconciler reconciles a ChaosExperiment object
type ChaosExperimentReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=chaos.shanto.dev,resources=chaosexperiments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=chaos.shanto.dev,resources=chaosexperiments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=chaos.shanto.dev,resources=chaosexperiments/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main Kubernetes reconciliation loop that aims to
// move the current state of the cluster closer to the desired state by
// performing operations to make the cluster state reflect the state specified by
// the ChaosExperiment object.
func (r *ChaosExperimentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the ChaosExperiment instance
	experiment := &chaosv1alpha1.ChaosExperiment{}
	err := r.Get(ctx, req.NamespacedName, experiment)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic,
			// use finalizers. Return and don't requeue
			logger.Info("ChaosExperiment resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get ChaosExperiment")
		return ctrl.Result{}, err
	}

	// Initialize experiment phase if it's empty
	if experiment.Status.Phase == "" {
		experiment.Status.Phase = chaosv1alpha1.ExperimentPending
		experiment.Status.Message = "Experiment initialized and pending."
		if err := r.Status().Update(ctx, experiment); err != nil {
			logger.Error(err, "Failed to update ChaosExperiment status to Pending")
			return ctrl.Result{}, err
		}
		r.Recorder.Event(experiment, "Normal", "ExperimentInitialized", "ChaosExperiment is initialized.")
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil // Requeue to start processing
	}

	// Handle "Completed" or "Failed" experiments
	if experiment.Status.Phase == chaosv1alpha1.ExperimentCompleted || experiment.Status.Phase == chaosv1alpha1.ExperimentFailed {
		if experiment.Spec.Mode == chaosv1alpha1.OneShotMode {
			logger.Info("One-shot experiment is completed or failed, not re-queueing", "Experiment", experiment.Name, "Phase", experiment.Status.Phase)
			return ctrl.Result{}, nil
		}
		// For recurring, we will requeue based on duration
		if experiment.Spec.Duration != nil {
			requeueAfter := experiment.Spec.Duration.Duration
			if experiment.Status.LastRunTime != nil {
				sinceLastRun := time.Since(experiment.Status.LastRunTime.Time)
				if sinceLastRun < requeueAfter {
					logger.Info("Recurring experiment completed, re-queueing for next run", "Experiment", experiment.Name, "RequeueAfter", requeueAfter-sinceLastRun)
					return ctrl.Result{RequeueAfter: requeueAfter - sinceLastRun}, nil
				}
			}
			logger.Info("Recurring experiment completed, immediately re-queueing for next run", "Experiment", experiment.Name)
			// Reset status for next run if it's recurring and duration has passed
			experiment.Status.Phase = chaosv1alpha1.ExperimentRunning // Or Pending, depending on desired behavior
			experiment.Status.Message = "Recurring experiment re-triggered."
			if err := r.Status().Update(ctx, experiment); err != nil {
				logger.Error(err, "Failed to update ChaosExperiment status for recurring run")
				return ctrl.Result{}, err
			}
			r.Recorder.Event(experiment, "Normal", "ExperimentReTriggered", "Recurring ChaosExperiment re-triggered.")
		}
	}

	// Check if the experiment should be completed based on duration
	if experiment.Spec.Duration != nil && experiment.Status.LastRunTime != nil {
		durationElapsed := time.Since(experiment.Status.LastRunTime.Time)
		if durationElapsed >= experiment.Spec.Duration.Duration {
			if experiment.Spec.Mode == chaosv1alpha1.OneShotMode {
				experiment.Status.Phase = chaosv1alpha1.ExperimentCompleted
				experiment.Status.Message = "Experiment completed successfully."
				if err := r.Status().Update(ctx, experiment); err != nil {
					logger.Error(err, "Failed to update ChaosExperiment status to Completed")
					return ctrl.Result{}, err
				}
				r.Recorder.Event(experiment, "Normal", "ExperimentCompleted", "ChaosExperiment has completed its one-shot execution.")
				return ctrl.Result{}, nil
			}
		}
	}

	// Perform the attack based on attack type
	switch experiment.Spec.Attack.Type {
	case chaosv1alpha1.PodKillAttack:
		return r.reconcilePodKillAttack(ctx, experiment)
	default:
		experiment.Status.Phase = chaosv1alpha1.ExperimentFailed
		experiment.Status.Message = "Unsupported attack type."
		if err := r.Status().Update(ctx, experiment); err != nil {
			logger.Error(err, "Failed to update ChaosExperiment status for unsupported attack type")
		}
		r.Recorder.Event(experiment, "Warning", "UnsupportedAttackType", "ChaosExperiment specified an unsupported attack type.")
		return ctrl.Result{}, nil
	}
}

func (r *ChaosExperimentReconciler) reconcilePodKillAttack(ctx context.Context, experiment *chaosv1alpha1.ChaosExperiment) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("AttackType", "PodKill")

	// 1. List pods in spec.target.namespace using the given labelSelector.
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(experiment.Spec.Target.Namespace),
		client.MatchingLabels(experiment.Spec.Target.LabelSelector),
	}
	if err := r.List(ctx, podList, listOpts...); err != nil {
		logger.Error(err, "Failed to list pods for chaos experiment", "Namespace", experiment.Spec.Target.Namespace, "LabelSelector", experiment.Spec.Target.LabelSelector)
		experiment.Status.Phase = chaosv1alpha1.ExperimentFailed
		experiment.Status.Message = "Failed to list target pods."
		r.Recorder.Event(experiment, "Warning", "PodListFailed", "Failed to list target pods.")
		if err := r.Status().Update(ctx, experiment); err != nil {
			logger.Error(err, "Failed to update ChaosExperiment status to Failed after pod listing error")
		}
		return ctrl.Result{RequeueAfter: time.Second * 30}, err // Requeue to retry listing pods
	}

	if len(podList.Items) == 0 {
		// No pods found, update status and requeue after some time.
		logger.Info("No target pods found for chaos experiment", "Namespace", experiment.Spec.Target.Namespace, "LabelSelector", experiment.Spec.Target.LabelSelector)
		experiment.Status.Phase = chaosv1alpha1.ExperimentFailed
		experiment.Status.Message = "No target pods found matching the label selector."
		r.Recorder.Event(experiment, "Warning", "NoTargetPods", "No target pods found for the experiment.")
		if err := r.Status().Update(ctx, experiment); err != nil {
			logger.Error(err, "Failed to update ChaosExperiment status to Failed after no pods found")
		}
		return ctrl.Result{RequeueAfter: time.Second * 60}, nil // Requeue to check again later
	}

	// 2. Pick one at random and delete it.
	r.seedRand() // Seed the random number generator
	podToKill := podList.Items[rand.Intn(len(podList.Items))]
	logger.Info("Attempting to delete pod", "PodName", podToKill.Name, "Namespace", podToKill.Namespace)

	if err := r.Delete(ctx, &podToKill); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Pod to kill not found, it might have been deleted already", "PodName", podToKill.Name)
			// Continue as if deleted, update status.
		} else {
			logger.Error(err, "Failed to delete pod", "PodName", podToKill.Name)
			experiment.Status.Phase = chaosv1alpha1.ExperimentFailed
			experiment.Status.Message = "Failed to delete target pod."
			r.Recorder.Eventf(experiment, "Warning", "PodDeletionFailed", "Failed to delete pod %s/%s", podToKill.Namespace, podToKill.Name)
			if err := r.Status().Update(ctx, experiment); err != nil {
				logger.Error(err, "Failed to update ChaosExperiment status to Failed after pod deletion error")
			}
			return ctrl.Result{RequeueAfter: time.Second * 30}, err // Requeue to retry
		}
	} else {
		logger.Info("Successfully deleted pod", "PodName", podToKill.Name)
		r.Recorder.Eventf(experiment, "Normal", "PodKilled", "Pod %s/%s was successfully killed.", podToKill.Namespace, podToKill.Name)
	}

	// 3. Set status.phase = "Running" and status.lastRunTime = now.
	experiment.Status.Phase = chaosv1alpha1.ExperimentRunning
	now := metav1.Now()
	experiment.Status.LastRunTime = &now
	experiment.Status.Message = "Pod-kill attack executed."

	if err := r.Status().Update(ctx, experiment); err != nil {
		logger.Error(err, "Failed to update ChaosExperiment status after pod kill")
		return ctrl.Result{}, err
	}

	// Determine next requeue for recurring experiments or for duration check
	if experiment.Spec.Mode == chaosv1alpha1.RecurringMode && experiment.Spec.Duration != nil {
		logger.Info("Requeuing recurring experiment", "Experiment", experiment.Name, "RequeueAfter", experiment.Spec.Duration.Duration)
		return ctrl.Result{RequeueAfter: experiment.Spec.Duration.Duration}, nil
	} else if experiment.Spec.Duration != nil {
		// For one-shot, requeue to check for completion if duration is set
		timeToCompletion := experiment.Spec.Duration.Duration - time.Since(experiment.Status.LastRunTime.Time)
		if timeToCompletion > 0 {
			logger.Info("Requeuing one-shot experiment to check for completion", "Experiment", experiment.Name, "RequeueAfter", timeToCompletion)
			return ctrl.Result{RequeueAfter: timeToCompletion}, nil
		}
	}

	// If one-shot and no duration, it's considered complete after one successful run
	if experiment.Spec.Mode == chaosv1alpha1.OneShotMode && experiment.Spec.Duration == nil {
		experiment.Status.Phase = chaosv1alpha1.ExperimentCompleted
		experiment.Status.Message = "One-shot experiment completed successfully (no duration specified)."
		if err := r.Status().Update(ctx, experiment); err != nil {
			logger.Error(err, "Failed to update ChaosExperiment status to Completed for one-shot without duration")
		}
		r.Recorder.Event(experiment, "Normal", "ExperimentCompleted", "One-shot ChaosExperiment completed successfully.")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// seedRand seeds the random number generator if it hasn't been seeded yet.
// This is important to ensure truly random pod selection across reconciles.
func (r *ChaosExperimentReconciler) seedRand() {
	// math/rand is automatically seeded in Go 1.20+
}

// SetupWithManager sets up the controller with the Manager.
func (r *ChaosExperimentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor("chaos-operator")
	return ctrl.NewControllerManagedBy(mgr).
		For(&chaosv1alpha1.ChaosExperiment{}).
		Owns(&corev1.Pod{}). // Watch for changes in Pods (e.g., deletions)
		Complete(r)
}
