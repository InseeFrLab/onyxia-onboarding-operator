/*
Copyright 2023.

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
	"fmt"
	"time"

	onyxiav1 "github.com/inseefrlab/onyxia-onboarding-operator/api/v1"
	"github.com/inseefrlab/onyxia-onboarding-operator/controllers/s3/factory"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	S3Config *factory.S3Config
}

//+kubebuilder:rbac:groups=onyxia.onyxia.sh,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=onyxia.onyxia.sh,resources=workspaces/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=onyxia.onyxia.sh,resources=workspaces/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Workspace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	onyxiaWorkspace := &onyxiav1.Workspace{}

	err := r.Get(ctx, req.NamespacedName, onyxiaWorkspace)

	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("Workspace %s has been removed. NOOP", req.Name))
		}
	} else {
		logger.Info("OnyxiaWorskpace to reconcile: " + fmt.Sprintf("%b", &onyxiaWorkspace))
		namespaceConfiguration := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: onyxiaWorkspace.Spec.Namespace},
		}

		err = handleBucket(onyxiaWorkspace, r.S3Config)
		if err != nil {
			log.Log.Error(err, err.Error())
			meta.SetStatusCondition(&onyxiaWorkspace.Status.Conditions,
				metav1.Condition{
					Type:               "OperatorDegraded",
					Status:             metav1.ConditionFalse,
					Reason:             "ReasonFailed",
					LastTransitionTime: metav1.NewTime(time.Now()),
					Message:            err.Error(),
					ObservedGeneration: onyxiaWorkspace.GetGeneration(),
				})
			return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, onyxiaWorkspace)})
		}

		err = r.Create(ctx, namespaceConfiguration)
		err = client.IgnoreAlreadyExists(err)
		if err != nil {
			log.Log.Error(err, err.Error())
			meta.SetStatusCondition(&onyxiaWorkspace.Status.Conditions,
				metav1.Condition{
					Type:               "OperatorDegraded",
					Status:             metav1.ConditionFalse,
					Reason:             "ReasonFailed",
					LastTransitionTime: metav1.NewTime(time.Now()),
					Message:            "failed to create namespace",
					ObservedGeneration: onyxiaWorkspace.GetGeneration(),
				})
			onyxiaWorkspace.Status.ObservedGeneration = onyxiaWorkspace.GetGeneration()
			return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, onyxiaWorkspace)})
		}
		logger.Info("Created / updated namespace", "namespace", onyxiaWorkspace.Namespace)
		onyxiaWorkspace.Status.ObservedGeneration = onyxiaWorkspace.GetGeneration()
		meta.SetStatusCondition(&onyxiaWorkspace.Status.Conditions, metav1.Condition{
			Type:               "OperatorSuccess",
			Status:             metav1.ConditionTrue,
			Reason:             "ReasonSucceeded",
			LastTransitionTime: metav1.NewTime(time.Now()),
			Message:            "operator successfully reconciling",
			ObservedGeneration: onyxiaWorkspace.GetGeneration(),
		})
		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, onyxiaWorkspace)})
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&onyxiav1.Workspace{}).
		Complete(r)
}

func handleBucket(onyxiaWorkspace *onyxiav1.Workspace, S3Config *factory.S3Config) error {
	//create bucket
	s3Client, err := factory.GetS3Client(S3Config.S3Provider, S3Config)
	if err != nil {
		log.Log.Error(err, err.Error())
		return fmt.Errorf("can't create an s3 client")
	}
	found, err := s3Client.BucketExists(onyxiaWorkspace.Spec.Bucket.Name)
	if err != nil {
		log.Log.Error(err, err.Error())
		return fmt.Errorf("can't create bucket " + onyxiaWorkspace.Spec.Bucket.Name)
	}
	if !found {
		err = s3Client.CreateBucket(onyxiaWorkspace.Spec.Bucket.Name)
		if err != nil {
			log.Log.Error(err, err.Error())
			return fmt.Errorf("can't create bucket " + onyxiaWorkspace.Spec.Bucket.Name)
		}
		err = s3Client.SetQuota(onyxiaWorkspace.Spec.Bucket.Name, onyxiaWorkspace.Spec.Bucket.Quota)
		if err != nil {
			log.Log.Error(err, err.Error())
			return fmt.Errorf("can't set quota for bucket " + onyxiaWorkspace.Spec.Bucket.Name)
		}
	} else {
		quota, err := s3Client.GetQuota(onyxiaWorkspace.Spec.Bucket.Name)
		if err != nil {
			log.Log.Error(err, err.Error())
			return fmt.Errorf("can't get quota for " + onyxiaWorkspace.Spec.Bucket.Name)
		}
		if quota != onyxiaWorkspace.Spec.Bucket.Quota {
			err = s3Client.SetQuota(onyxiaWorkspace.Spec.Bucket.Name, onyxiaWorkspace.Spec.Bucket.Quota)
			if err != nil {
				log.Log.Error(err, err.Error())
				return fmt.Errorf("can't set quota for " + onyxiaWorkspace.Spec.Bucket.Name)
			}
		}
	}
	return nil
}
