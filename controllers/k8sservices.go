package controllers

import (
	"context"

	grpcdemov1 "github.com/drhelius/grpc-demo-operator/api/v1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/deprecated/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *DemoServicesReconciler) manageService(instance *grpcdemov1.DemoServices, srv grpcdemov1.Service, reqLogger logr.Logger) error {

	new := newService(srv, instance)

	if err := controllerutil.SetControllerReference(instance, new, r.Scheme); err != nil {
		reqLogger.Error(err, "Failed to set controller reference", "Service", new)
		return err
	}

	found := &corev1.Service{}

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: new.GetName(), Namespace: new.GetNamespace()}, found)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new service", "Service", new)

		err := r.Client.Create(context.TODO(), new)

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

			err := r.Client.Update(context.TODO(), new)

			if err != nil {
				reqLogger.Error(err, "Failed to update service", "Service", new)
				return err
			}
		}
	}

	return nil
}

func (r *DemoServicesReconciler) deleteOrphanedServices(instance *grpcdemov1.DemoServices, reqLogger logr.Logger) error {

	selector := map[string]string{
		"group": instance.ObjectMeta.Name,
	}

	listOps := &client.ListOptions{
		Namespace:     instance.Namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}

	serviceList := &corev1.ServiceList{}
	err := r.Client.List(context.TODO(), serviceList, listOps)
	if err != nil {
		reqLogger.Error(err, "Failed to list services")
		return err
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
			r.Client.Delete(context.TODO(), &service)
			if err != nil {
				reqLogger.Error(err, "Failed to delete service", "Service", service)
				return err
			}
		}
	}

	return nil
}

func newService(srv grpcdemov1.Service, cr *grpcdemov1.DemoServices) *corev1.Service {

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
