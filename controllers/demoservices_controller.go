/*


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

	grpcdemov1 "github.com/drhelius/grpc-demo-operator/api/v1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// DemoServicesReconciler reconciles a DemoServices object
type DemoServicesReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=grpcdemo.example.com,resources=demoservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=grpcdemo.example.com,resources=demoservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=grpcdemo.example.com,resources=demoservices/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete

func (r *DemoServicesReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {

	reqLogger := r.Log.WithValues("req.Namespace", req.Namespace, "req.Name", req.Name)

	reqLogger.Info("Reconciling Services")

	instance := &grpcdemov1.DemoServices{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	for _, srv := range instance.Spec.Services {
		err := r.manageDeployment(instance, srv, reqLogger)
		if err != nil {
			return reconcile.Result{}, err
		}

		err = r.manageService(instance, srv, reqLogger)
		if err != nil {
			return reconcile.Result{}, err
		}

		err = r.manageRoute(instance, srv, reqLogger)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	err = r.deleteOrphanedDeployments(instance, reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = r.deleteOrphanedServices(instance, reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = r.deleteOrphanedRoutes(instance, reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *DemoServicesReconciler) SetupWithManager(mgr ctrl.Manager) error {

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&grpcdemov1.DemoServices{}).
		Build(r)
	if err != nil {
		return err
	}

	predCR := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.MetaOld.GetGeneration() != e.MetaNew.GetGeneration()
		},
	}

	err = c.Watch(&source.Kind{Type: &grpcdemov1.DemoServices{}}, &handler.EnqueueRequestForObject{}, predCR)
	if err != nil {
		return err
	}

	h := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &grpcdemov1.DemoServices{},
	}

	predDeployment := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.MetaOld.GetGeneration() != e.MetaNew.GetGeneration()
		},
	}

	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, h, predDeployment)
	if err != nil {
		return err
	}

	predService := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.MetaOld.GetResourceVersion() != e.MetaNew.GetResourceVersion()
		},
	}

	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, h, predService)
	if err != nil {
		return err
	}

	predRoute := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldRoute, ok := e.ObjectOld.(*routev1.Route)
			if !ok {
				return false
			}
			newRoute, ok := e.ObjectNew.(*routev1.Route)
			if !ok {
				return false
			}

			if oldRoute.Spec.Host == "" {
				return false
			}

			return cmp.Equal(oldRoute.Status.Ingress, newRoute.Status.Ingress)
		},
	}

	err = c.Watch(&source.Kind{Type: &routev1.Route{}}, h, predRoute)
	if err != nil {
		return err
	}

	return nil
}
