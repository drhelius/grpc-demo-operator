package services

import (
	"context"

	grpcdemov1 "github.com/drhelius/grpc-demo-operator/pkg/apis/grpcdemo/v1"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/deprecated/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

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

func (r *ReconcileServices) deleteOrphanedDeployments(instance *grpcdemov1.Services, reqLogger logr.Logger) error {

	selector := map[string]string{
		"group": instance.ObjectMeta.Name,
	}

	listOps := &client.ListOptions{
		Namespace:     instance.Namespace,
		LabelSelector: labels.SelectorFromSet(selector),
	}

	depList := &appsv1.DeploymentList{}
	err := r.client.List(context.TODO(), depList, listOps)
	if err != nil {
		reqLogger.Error(err, "Failed to list deployemnts")
		return err
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
