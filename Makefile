OPERATOR_IMAGE=quay.io/isanchez/grpc-demo-operator
OPERATOR_VERSION=v0.0.1

all:generate

setup:
	kubectl apply -f deploy/crds/grpcdemo.example.com_services_crd.yaml
	kubectl apply -f deploy/service_account.yaml
	kubectl apply -f deploy/role.yaml
	kubectl apply -f deploy/role_binding.yaml
	kubectl apply -f deploy/operator.yaml

push:
	docker push $(OPERATOR_IMAGE):$(OPERATOR_VERSION)

generate:
	operator-sdk generate k8s
	operator-sdk generate crds
	operator-sdk build $(OPERATOR_IMAGE):$(OPERATOR_VERSION)