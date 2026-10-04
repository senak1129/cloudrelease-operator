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
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	deliveryv1alpha1 "github.com/senak1129/cloudrelease-operator/api/v1alpha1"
)

const (
	releaseUIDLabel                        = "delivery.cloudrelease.dev/release-uid"
	managedByLabel                         = "app.kubernetes.io/managed-by"
	managedByValue                         = "cloudrelease-operator"
	conditionAvailable                     = "Available"
	conditionProgressing                   = "Progressing"
	conditionDegraded                      = "Degraded"
	progressDeadlineExceededReason         = "ProgressDeadlineExceeded"
	deploymentAvailableReason              = "DeploymentAvailable"
	deploymentProgressingReason            = "DeploymentProgressing"
	deploymentDegradedReason               = "DeploymentDegraded"
	deploymentWithinProgressDeadlineReason = "DeploymentWithinProgressDeadline"
)

// CloudReleaseReconciler reconciles a CloudRelease object
type CloudReleaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=delivery.cloudrelease.dev,resources=cloudreleases,verbs=get;list;watch
// +kubebuilder:rbac:groups=delivery.cloudrelease.dev,resources=cloudreleases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the CloudRelease object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *CloudReleaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling CloudRelease")

	release := &deliveryv1alpha1.CloudRelease{}
	if err := r.Get(ctx, req.NamespacedName, release); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("CloudRelease no longer exists")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("get CloudRelease %s: %w", req.NamespacedName, err)
	}

	log.Info(
		"Loaded CloudRelease",
		"generation", release.Generation,
		"image", release.Spec.Image,
		"replicas", release.Spec.Replicas,
		"port", release.Spec.Port,
	)

	deployment, operation, err := r.reconcileDeployment(ctx, release)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile Deployment: %w", err)
	}

	log.Info("Reconciled Deployment", "operation", operation)

	operation, err = r.reconcileService(ctx, release)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile Service: %w", err)
	}

	log.Info("Reconciled Service", "operation", operation)

	statusUpdated, err := r.reconcileStatus(ctx, release, deployment)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile CloudRelease status: %w", err)
	}

	log.Info(
		"Reconciled CloudRelease status",
		"updated", statusUpdated,
		"observedGeneration", release.Status.ObservedGeneration,
		"readyReplicas", release.Status.ReadyReplicas,
	)

	return ctrl.Result{}, nil
}

func (r *CloudReleaseReconciler) reconcileDeployment(
	ctx context.Context,
	release *deliveryv1alpha1.CloudRelease,
) (*appsv1.Deployment, controllerutil.OperationResult, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      release.Name,
			Namespace: release.Namespace,
		},
	}

	operation, err := controllerutil.CreateOrPatch(ctx, r.Client, deployment, func() error {
		if deployment.ResourceVersion != "" && !metav1.IsControlledBy(deployment, release) {
			return fmt.Errorf(
				"Deployment %s/%s already exists and is not controlled by CloudRelease %s/%s",
				deployment.Namespace,
				deployment.Name,
				release.Namespace,
				release.Name,
			)
		}

		if err := controllerutil.SetControllerReference(release, deployment, r.Scheme); err != nil {
			return fmt.Errorf("set CloudRelease as Deployment controller: %w", err)
		}

		setManagedLabels(deployment, map[string]string{
			releaseUIDLabel: string(release.UID),
			managedByLabel:  managedByValue,
		})

		desiredSelector := &metav1.LabelSelector{
			MatchLabels: map[string]string{
				releaseUIDLabel: string(release.UID),
			},
		}

		if deployment.ResourceVersion == "" {
			deployment.Spec.Selector = desiredSelector
		} else if !apiequality.Semantic.DeepEqual(deployment.Spec.Selector, desiredSelector) {
			return fmt.Errorf(
				"Deployment %s/%s has immutable selector %v, expected %v",
				deployment.Namespace,
				deployment.Name,
				deployment.Spec.Selector,
				desiredSelector,
			)
		}

		replicas := release.Spec.Replicas
		deployment.Spec.Replicas = &replicas

		if deployment.Spec.Template.Labels == nil {
			deployment.Spec.Template.Labels = make(map[string]string)
		}
		deployment.Spec.Template.Labels[releaseUIDLabel] = string(release.UID)
		deployment.Spec.Template.Labels[managedByLabel] = managedByValue

		deployment.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:                     "application",
				Image:                    release.Spec.Image,
				ImagePullPolicy:          corev1.PullIfNotPresent,
				TerminationMessagePath:   corev1.TerminationMessagePathDefault,
				TerminationMessagePolicy: corev1.TerminationMessageReadFile,
				Ports: []corev1.ContainerPort{
					{
						Name:          "http",
						ContainerPort: release.Spec.Port,
						Protocol:      corev1.ProtocolTCP,
					},
				},
			},
		}

		return nil
	})

	return deployment, operation, err
}

func (r *CloudReleaseReconciler) reconcileService(
	ctx context.Context,
	release *deliveryv1alpha1.CloudRelease,
) (controllerutil.OperationResult, error) {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      release.Name,
			Namespace: release.Namespace,
		},
	}

	return controllerutil.CreateOrPatch(ctx, r.Client, service, func() error {
		if service.ResourceVersion != "" && !metav1.IsControlledBy(service, release) {
			return fmt.Errorf(
				"Service %s/%s already exists and is not controlled by CloudRelease %s/%s",
				service.Namespace,
				service.Name,
				release.Namespace,
				release.Name,
			)
		}

		if err := controllerutil.SetControllerReference(release, service, r.Scheme); err != nil {
			return fmt.Errorf("set CloudRelease as Service controller: %w", err)
		}

		setManagedLabels(service, map[string]string{
			releaseUIDLabel: string(release.UID),
			managedByLabel:  managedByValue,
		})

		service.Spec.Type = corev1.ServiceTypeClusterIP
		service.Spec.Selector = map[string]string{
			releaseUIDLabel: string(release.UID),
		}
		service.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "http",
				Protocol:   corev1.ProtocolTCP,
				Port:       release.Spec.Port,
				TargetPort: intstr.FromString("http"),
			},
		}

		return nil
	})
}

func (r *CloudReleaseReconciler) reconcileStatus(
	ctx context.Context,
	release *deliveryv1alpha1.CloudRelease,
	deployment *appsv1.Deployment,
) (bool, error) {
	before := release.DeepCopy()

	release.Status.ObservedGeneration = release.Generation
	release.Status.ReadyReplicas = deployment.Status.ReadyReplicas

	desiredReplicas := release.Spec.Replicas
	available := deployment.Status.ObservedGeneration == deployment.Generation &&
		deployment.Status.UpdatedReplicas == desiredReplicas &&
		deployment.Status.ReadyReplicas == desiredReplicas &&
		deployment.Status.UnavailableReplicas == 0
	degraded := deploymentProgressDeadlineExceeded(deployment)

	if available {
		meta.SetStatusCondition(&release.Status.Conditions, metav1.Condition{
			Type:               conditionAvailable,
			Status:             metav1.ConditionTrue,
			ObservedGeneration: release.Generation,
			Reason:             deploymentAvailableReason,
			Message:            fmt.Sprintf("Deployment has %d ready replicas", deployment.Status.ReadyReplicas),
		})
		meta.SetStatusCondition(&release.Status.Conditions, metav1.Condition{
			Type:               conditionProgressing,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: release.Generation,
			Reason:             deploymentAvailableReason,
			Message:            "Deployment rollout is complete",
		})
	} else {
		availableReason := deploymentProgressingReason
		availableMessage := fmt.Sprintf(
			"Waiting for Deployment rollout: %d/%d replicas ready",
			deployment.Status.ReadyReplicas,
			desiredReplicas,
		)
		progressingStatus := metav1.ConditionTrue
		progressingReason := deploymentProgressingReason
		progressingMessage := availableMessage

		if degraded {
			availableReason = deploymentDegradedReason
			availableMessage = "Deployment exceeded its progress deadline"
			progressingStatus = metav1.ConditionFalse
			progressingReason = deploymentDegradedReason
			progressingMessage = availableMessage
		}

		meta.SetStatusCondition(&release.Status.Conditions, metav1.Condition{
			Type:               conditionAvailable,
			Status:             metav1.ConditionFalse,
			ObservedGeneration: release.Generation,
			Reason:             availableReason,
			Message:            availableMessage,
		})
		meta.SetStatusCondition(&release.Status.Conditions, metav1.Condition{
			Type:               conditionProgressing,
			Status:             progressingStatus,
			ObservedGeneration: release.Generation,
			Reason:             progressingReason,
			Message:            progressingMessage,
		})
	}

	degradedStatus := metav1.ConditionFalse
	degradedReason := deploymentWithinProgressDeadlineReason
	degradedMessage := "Deployment has not exceeded its progress deadline"
	if degraded {
		degradedStatus = metav1.ConditionTrue
		degradedReason = deploymentDegradedReason
		degradedMessage = "Deployment exceeded its progress deadline"
	}

	meta.SetStatusCondition(&release.Status.Conditions, metav1.Condition{
		Type:               conditionDegraded,
		Status:             degradedStatus,
		ObservedGeneration: release.Generation,
		Reason:             degradedReason,
		Message:            degradedMessage,
	})

	if apiequality.Semantic.DeepEqual(before.Status, release.Status) {
		return false, nil
	}

	if err := r.Status().Patch(ctx, release, client.MergeFrom(before)); err != nil {
		return false, fmt.Errorf("patch status: %w", err)
	}

	return true, nil
}

func deploymentProgressDeadlineExceeded(deployment *appsv1.Deployment) bool {
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing &&
			condition.Status == corev1.ConditionFalse &&
			condition.Reason == progressDeadlineExceededReason {
			return true
		}
	}

	return false
}

func setManagedLabels(object client.Object, desired map[string]string) {
	labels := object.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	for key, value := range desired {
		labels[key] = value
	}

	object.SetLabels(labels)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CloudReleaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&deliveryv1alpha1.CloudRelease{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("cloudrelease").
		Complete(r)
}
