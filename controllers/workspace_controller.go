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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// WorkspaceReconciler reconciles a Workspace object
type WorkspaceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	S3Client *factory.S3Client
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

	//crl.createOrUpdate
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("Workspace %s has been removed. NOOP", req.Name))
			//should clean up here
			return ctrl.Result{}, nil
		}
	}
	// logger.Info("OnyxiaWorskpace to reconcile: " + fmt.Sprintf("%b", &onyxiaWorkspace))

	err = handleBucket(onyxiaWorkspace, *r.S3Client)
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
	namespaceConfiguration := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: onyxiaWorkspace.Spec.Namespace},
	}
	//cluster-scoped resource must not have a namespace-scoped owne
	//err = ctrl.SetControllerReference(onyxiaWorkspace, namespaceConfiguration, r.Scheme)
	if err != nil {
		log.Log.Error(err, err.Error())
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
	err = r.addResourceQuotaToNamespace(r.Client, onyxiaWorkspace)
	err = client.IgnoreAlreadyExists(err)
	if err != nil {
		log.Log.Error(err, err.Error())
		meta.SetStatusCondition(&onyxiaWorkspace.Status.Conditions,
			metav1.Condition{
				Type:               "OperatorDegraded",
				Status:             metav1.ConditionFalse,
				Reason:             "ReasonFailed",
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            "failed to put resourcequota",
				ObservedGeneration: onyxiaWorkspace.GetGeneration(),
			})
		onyxiaWorkspace.Status.ObservedGeneration = onyxiaWorkspace.GetGeneration()
		return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, onyxiaWorkspace)})
	}
	logger.Info("Created / updated namespace", "namespace", onyxiaWorkspace.Namespace)
	onyxiaWorkspace.Status.ObservedGeneration = onyxiaWorkspace.GetGeneration()
	meta.SetStatusCondition(&onyxiaWorkspace.Status.Conditions, metav1.Condition{
		Type:               "OperatorSuccessful",
		Status:             metav1.ConditionTrue,
		Reason:             "ReasonSucceeded",
		LastTransitionTime: metav1.NewTime(time.Now()),
		Message:            "operator successfully reconciling",
		ObservedGeneration: onyxiaWorkspace.GetGeneration(),
	})
	return ctrl.Result{}, utilerrors.NewAggregate([]error{err, r.Status().Update(ctx, onyxiaWorkspace)})

}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&onyxiav1.Workspace{}).
		// WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: resourceKindPredicate{}.Create,
			UpdateFunc: resourceGenerationOrFinalizerChangedPredicate{}.Update,
			//DeleteFunc:  deleteFunc,
			//	GenericFunc: genericFunc
		}).
		//Owns(&v1.Namespace{}).
		// Owns(&v1.ResourceQuota{}).
		Complete(r)
}

func handleBucket(onyxiaWorkspace *onyxiav1.Workspace, s3Client factory.S3Client) error {
	//create bucket
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
		for _, v := range onyxiaWorkspace.Spec.Bucket.Paths {
			err = s3Client.CreatePath(onyxiaWorkspace.Spec.Bucket.Name, v)
			if err != nil {
				log.Log.Error(err, err.Error())
				return fmt.Errorf("can't create path " + v)
			}
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
		for _, v := range onyxiaWorkspace.Spec.Bucket.Paths {
			found, err = s3Client.PathExists(onyxiaWorkspace.Spec.Bucket.Name, v)
			if err != nil {
				log.Log.Error(err, err.Error())
				return fmt.Errorf("can't check path " + onyxiaWorkspace.Spec.Bucket.Name)
			}
			if !found {
				err = s3Client.CreatePath(onyxiaWorkspace.Spec.Bucket.Name, v)
				if err != nil {
					log.Log.Error(err, err.Error())
					return fmt.Errorf("can't create path " + v)
				}
			}
		}
	}
	return nil
}

func (r *WorkspaceReconciler) addResourceQuotaToNamespace(c client.Client, onyxiaWorkspace *onyxiav1.Workspace) error {
	// Créer un objet ResourceQuota
	mergedMap := onyxiaWorkspace.Spec.Quota.Default
	for k, v := range onyxiaWorkspace.Spec.Quota.Admin {
		mergedMap[k] = v
	}
	resourceLimit := v1.ResourceList{}

	for k, v := range mergedMap {
		fmt.Println(k)
		switch k {
		case v1.ResourcePods.String():
			resourceLimit[v1.ResourcePods] = resource.MustParse(v)
		case v1.ResourceRequestsCPU.String():
			resourceLimit[v1.ResourceRequestsCPU] = resource.MustParse(v)
		case v1.ResourceRequestsMemory.String():
			resourceLimit[v1.ResourceRequestsMemory] = resource.MustParse(v)
		case v1.ResourceLimitsCPU.String():
			fmt.Println(v)
			resourceLimit[v1.ResourceLimitsCPU] = resource.MustParse(v)
		case v1.ResourceLimitsMemory.String():
			fmt.Println(v)
			resourceLimit[v1.ResourceLimitsMemory] = resource.MustParse(v)
		case v1.ResourceRequestsStorage.String():
			resourceLimit[v1.ResourceRequestsStorage] = resource.MustParse(v)

		default:

		}
	}

	quota := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "quota-" + onyxiaWorkspace.Name,
			Namespace: onyxiaWorkspace.Namespace,
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: resourceLimit,
		},
	}
	// ctrl.CreateOrUpdate
	// set owner reference to recreate quota
	ctrl.SetControllerReference(onyxiaWorkspace, quota, r.Scheme)
	controllerutil.AddFinalizer(quota, "onyxia.onboarding/finalizer")
	//on crée
	err := c.Create(context.Background(), quota)
	if apierrors.IsAlreadyExists(err) {
		//on update
		c.Update(context.Background(), quota)
	} else {
		err = client.IgnoreAlreadyExists(err)
		if err != nil {
			return fmt.Errorf("failed to create ResourceQuota: %v", err)
		}
	}
	return nil
}
