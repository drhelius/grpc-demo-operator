module github.com/drhelius/grpc-demo-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/openshift/api v0.0.0-20200609191024-dca637550e8c
	github.com/operator-framework/operator-sdk v0.18.1
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.6.0
	github.com/google/go-cmp v0.4.1
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.18.2 // Required by prometheus-operator
)
