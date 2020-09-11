package controllers

import (
	"context"

	grpcdemov1 "github.com/drhelius/grpc-demo-operator/api/v1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	routev1 "github.com/openshift/api/route/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/deprecated/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DemoServicesReconciler) manageRoute(instance *grpcdemov1.DemoServices, srv grpcdemov1.Service, reqLogger logr.Logger) error {

	new := newRoute(srv, instance)

	if err := controllerutil.SetControllerReference(instance, new, r.Scheme); err != nil {
		reqLogger.Error(err, "Failed to set controller reference", "Route", new)
		return err
	}

	found := &routev1.Route{}

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: new.GetName(), Namespace: new.GetNamespace()}, found)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new route", "Route", new)

		err := r.Client.Create(context.TODO(), new)

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

			err := r.Client.Update(context.TODO(), new)

			if err != nil {
				reqLogger.Error(err, "Failed to update route", "Route", new)
				return err
			}
		}
	}

	return nil
}

func (r *DemoServicesReconciler) deleteOrphanedRoutes(instance *grpcdemov1.DemoServices, reqLogger logr.Logger) error {

	selector := map[string]string{
		"group": instance.ObjectMeta.Name,
	}

	listOps := &client.ListOptions{
		Namespace:     instance.Namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}

	routeList := &routev1.RouteList{}
	err := r.Client.List(context.TODO(), routeList, listOps)
	if err != nil {
		reqLogger.Error(err, "Failed to list routes")
		return err
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
			r.Client.Delete(context.TODO(), &route)
			if err != nil {
				reqLogger.Error(err, "Failed to delete route", "Route", route)
				return err
			}
		}
	}

	return nil
}

func newRoute(srv grpcdemov1.Service, cr *grpcdemov1.DemoServices) *routev1.Route {

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
