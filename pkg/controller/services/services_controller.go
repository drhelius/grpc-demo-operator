package services

import (
	"context"

	grpcdemov1 "github.com/drhelius/grpc-demo-operator/pkg/apis/grpcdemo/v1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

type kubernetesList struct {
	Items []runtime.Object
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

	selector := map[string]string{
		"group": instance.ObjectMeta.Name,
	}

	listOps := &client.ListOptions{
		Namespace:     instance.Namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}

	depList := &appsv1.DeploymentList{}
	err = r.client.List(context.TODO(), depList, listOps)
	if err != nil {
		reqLogger.Error(err, "Failed to list deployemnts")
		return reconcile.Result{}, err
	}

	for _, dep := range depList.Items {
		found := false

		for _, srv := range instance.Spec.Services {
			if dep.Name == srv.Name {
				found = true
			}
		}

		if !found {
			reqLogger.Info("Deleting unused deployment", "Deployment", dep)
			r.client.Delete(context.TODO(), &dep)
			if err != nil {
				reqLogger.Error(err, "Failed to delete deployemnt", "Deployment", dep)
				return reconcile.Result{}, err
			}
		}
	}

	serviceList := &corev1.ServiceList{}
	err = r.client.List(context.TODO(), serviceList, listOps)
	if err != nil {
		reqLogger.Error(err, "Failed to list services")
		return reconcile.Result{}, err
	}

	for _, service := range serviceList.Items {
		found := false

		for _, srv := range instance.Spec.Services {
			if service.Name == srv.Name {
				found = true
			}
		}

		if !found {
			reqLogger.Info("Deleting unused service", "Service", service)
			r.client.Delete(context.TODO(), &service)
			if err != nil {
				reqLogger.Error(err, "Failed to delete service", "Service", service)
				return reconcile.Result{}, err
			}
		}
	}

	routeList := &routev1.RouteList{}
	err = r.client.List(context.TODO(), routeList, listOps)
	if err != nil {
		reqLogger.Error(err, "Failed to list routes")
		return reconcile.Result{}, err
	}

	for _, route := range routeList.Items {
		found := false

		for _, srv := range instance.Spec.Services {
			if route.Name == srv.Name {
				found = true
			}
		}

		if !found {
			reqLogger.Info("Deleting unused route", "Route", route)
			r.client.Delete(context.TODO(), &route)
			if err != nil {
				reqLogger.Error(err, "Failed to delete route", "Route", route)
				return reconcile.Result{}, err
			}
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileServices) manageDeployment(instance *grpcdemov1.Services, srv grpcdemov1.Service, reqLogger logr.Logger) error {

	new := newDeployment(srv, instance)

	if err := controllerutil.SetControllerReference(instance, new, r.scheme); err != nil {
		reqLogger.Error(err, "Failed to set controller reference", "Deployment", new)
		return err
	}

	found := &appsv1.Deployment{}

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: new.GetName(), Namespace: new.GetNamespace()}, found)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new deployment", "Deployment", new)

		err := r.client.Create(context.TODO(), new)

		if err != nil {
			reqLogger.Error(err, "Failed to create deployemnt", "Deployment", new)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to find deployemnt", "Deployment", new)
		return err
	} else if !cmp.Equal(new.Spec, found.Spec) {
		reqLogger.Info("Updating deployment", "Deployment", new)

		err := r.client.Update(context.TODO(), new)

		if err != nil {
			reqLogger.Error(err, "Failed to update deployemnt", "Deployment", new)
			return err
		}
	}

	return nil
}

func (r *ReconcileServices) manageService(instance *grpcdemov1.Services, srv grpcdemov1.Service, reqLogger logr.Logger) error {

	new := newService(srv, instance)

	if err := controllerutil.SetControllerReference(instance, new, r.scheme); err != nil {
		reqLogger.Error(err, "Failed to set controller reference", "Service", new)
		return err
	}

	found := &corev1.Service{}

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: new.GetName(), Namespace: new.GetNamespace()}, found)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new service", "Service", new)

		err := r.client.Create(context.TODO(), new)

		if err != nil {
			reqLogger.Error(err, "Failed to create service", "Service", new)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to find service", "Service", new)
		return err
	} else {

		new.Spec.ClusterIP = found.Spec.ClusterIP
		new.ObjectMeta.ResourceVersion = found.ObjectMeta.ResourceVersion

		if !cmp.Equal(new.Spec, found.Spec) {
			reqLogger.Info("Updating service", "Service", new)

			err := r.client.Update(context.TODO(), new)

			if err != nil {
				reqLogger.Error(err, "Failed to update service", "Service", new)
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileServices) manageRoute(instance *grpcdemov1.Services, srv grpcdemov1.Service, reqLogger logr.Logger) error {

	new := newRoute(srv, instance)

	if err := controllerutil.SetControllerReference(instance, new, r.scheme); err != nil {
		reqLogger.Error(err, "Failed to set controller reference", "Route", new)
		return err
	}

	found := &routev1.Route{}

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: new.GetName(), Namespace: new.GetNamespace()}, found)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new route", "Route", new)

		err := r.client.Create(context.TODO(), new)

		if err != nil {
			reqLogger.Error(err, "Failed to create route", "Route", new)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to find route", "Route", new)
		return err
	} else {

		new.Spec.Host = found.Spec.Host
		new.ObjectMeta.ResourceVersion = found.ObjectMeta.ResourceVersion

		if !cmp.Equal(new.Spec, found.Spec) {
			reqLogger.Info("Updating route", "Route", new)

			new.Spec.Host = ""

			err := r.client.Update(context.TODO(), new)

			if err != nil {
				reqLogger.Error(err, "Failed to update route", "Route", new)
				return err
			}
		}
	}

	return nil
}

func newDeployment(srv grpcdemov1.Service, cr *grpcdemov1.Services) *appsv1.Deployment {

	labels := map[string]string{
		"app":                         srv.Name,
		"version":                     srv.Version,
		"app.kubernetes.io/name":      srv.Name,
		"app.kubernetes.io/version":   srv.Version,
		"app.kubernetes.io/component": "service",
		"app.kubernetes.io/part-of":   cr.ObjectMeta.Name,
		"app.openshift.io/runtime":    "golang",
		"group":                       cr.ObjectMeta.Name,
	}

	selector := map[string]string{
		"app":     srv.Name,
		"version": srv.Version,
	}

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      srv.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &srv.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: selector,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: selector,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           srv.Image,
						ImagePullPolicy: corev1.PullAlways,
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(8080)},
							},
							InitialDelaySeconds: 30,
							TimeoutSeconds:      1,
							PeriodSeconds:       30,
							SuccessThreshold:    1,
							FailureThreshold:    3,
						},
						Name: srv.Name,
						Ports: []corev1.ContainerPort{
							{ContainerPort: 5000, Name: "grpc", Protocol: corev1.ProtocolTCP},
							{ContainerPort: 8080, Name: "http", Protocol: corev1.ProtocolTCP},
						},
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								"cpu":    resource.MustParse(srv.Limits.CPU),
								"memory": resource.MustParse(srv.Limits.Memory),
							},
							Requests: corev1.ResourceList{
								"cpu":    resource.MustParse(srv.Requests.CPU),
								"memory": resource.MustParse(srv.Requests.Memory),
							},
						},
						ReadinessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt(8080)},
							},
							InitialDelaySeconds: 5,
							TimeoutSeconds:      1,
							PeriodSeconds:       10,
							SuccessThreshold:    1,
							FailureThreshold:    3,
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: "File",
					}},
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					SchedulerName:                 "default-scheduler",
					SecurityContext:               &corev1.PodSecurityContext{},
					TerminationGracePeriodSeconds: func(i int64) *int64 { return &i }(30),
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
					MaxSurge:       &intstr.IntOrString{Type: intstr.String, StrVal: "25%"},
				},
			},
			RevisionHistoryLimit:    func(i int32) *int32 { return &i }(10),
			ProgressDeadlineSeconds: func(i int32) *int32 { return &i }(600),
		},
	}

	scheme := scheme.Scheme
	scheme.Default(dep)

	return dep
}

func newService(srv grpcdemov1.Service, cr *grpcdemov1.Services) *corev1.Service {

	labels := map[string]string{
		"app":                         srv.Name,
		"version":                     srv.Version,
		"app.kubernetes.io/name":      srv.Name,
		"app.kubernetes.io/version":   srv.Version,
		"app.kubernetes.io/component": "service",
		"app.kubernetes.io/part-of":   cr.ObjectMeta.Name,
		"group":                       cr.ObjectMeta.Name,
	}

	selector := map[string]string{
		"app":     srv.Name,
		"version": srv.Version,
	}

	k8ssrv := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      srv.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "",
			Ports: []corev1.ServicePort{
				{Port: 5000, TargetPort: intstr.FromInt(5000), Name: "grpc", Protocol: corev1.ProtocolTCP},
				{Port: 8080, TargetPort: intstr.FromInt(8080), Name: "http", Protocol: corev1.ProtocolTCP},
			},
			Selector:        selector,
			SessionAffinity: corev1.ServiceAffinityNone,
			Type:            corev1.ServiceTypeClusterIP,
		},
	}

	scheme := scheme.Scheme
	scheme.Default(k8ssrv)

	return k8ssrv
}

func newRoute(srv grpcdemov1.Service, cr *grpcdemov1.Services) *routev1.Route {

	labels := map[string]string{
		"app":                         srv.Name,
		"version":                     srv.Version,
		"app.kubernetes.io/name":      srv.Name,
		"app.kubernetes.io/version":   srv.Version,
		"app.kubernetes.io/component": "service",
		"app.kubernetes.io/part-of":   cr.ObjectMeta.Name,
		"group":                       cr.ObjectMeta.Name,
	}

	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "route.openshift.io/v1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      srv.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: routev1.RouteSpec{
			Host: "",
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString("http"),
			},
			To: routev1.RouteTargetReference{
				Kind:   "Service",
				Name:   srv.Name,
				Weight: func(i int32) *int32 { return &i }(100),
			},
			WildcardPolicy: routev1.WildcardPolicyNone,
		},
	}

	scheme := scheme.Scheme
	scheme.Default(route)

	return route
}
