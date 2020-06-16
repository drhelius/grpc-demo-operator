package services

import (
	"context"

	grpcdemov1 "github.com/drhelius/grpc-demo-operator/pkg/apis/grpcdemo/v1"
	"github.com/google/go-cmp/cmp"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type kubernetesResource interface {
	metav1.Object
	runtime.Object
}

var log = logf.Log.WithName("controller_services")

// Add creates a new Services Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileServices{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("services-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	predCR := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.MetaOld.GetGeneration() != e.MetaNew.GetGeneration()
		},
	}

	err = c.Watch(&source.Kind{Type: &grpcdemov1.Services{}}, &handler.EnqueueRequestForObject{}, predCR)
	if err != nil {
		return err
	}

	h := &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &grpcdemov1.Services{},
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

var _ reconcile.Reconciler = &ReconcileServices{}

// ReconcileServices reconciles a Services object
type ReconcileServices struct {
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Services object and makes changes based on the state read
// and what is in the Services.Spec
func (r *ReconcileServices) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)

	reqLogger.Info("Reconciling Services")

	instance := &grpcdemov1.Services{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
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

	return reconcile.Result{}, nil
}
