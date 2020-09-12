# grpc-demo-operator

This repo is part of the [Istio gRPC Golang Demo](https://github.com/drhelius/grpc-demo), please refer to that project for documentation.

## Build

Clone the repo:

```bash
$ git clone https://github.com/drhelius/grpc-demo-operator.git
$ cd grpc-demo-operator
```

Build the operator and the Docker image:

```bash
$ make docker-build
```

## Run

Make sure you are working with the right namespace. The operator will run in the `grpc-demo` namespace by default:

`$ oc project grpc-demo`

Run this to build and deploy the operator, the CRDs and all required manifests like RBAC configuration:

```bash
$ make install
$ make deploy IMG=quay.io/isanchez/grpc-demo-operator:v0.0.1
```

Make sure the operator is running fine:

```bash
$ kubectl get deployment grpc-demo-operator-controller-manager
NAME                                    READY   UP-TO-DATE   AVAILABLE   AGE
grpc-demo-operator-controller-manager   1/1     1            1           2m56s
```

The operator is watching custom resources with kind `demoservices.grpcdemo.example.com`.

Write a [sample custom resource](config/samples/grpcdemo_v1_demoservices.yaml):

```yaml
apiVersion: grpcdemo.example.com/v1
kind: DemoServices
metadata:
  name: example-services
spec:
  services:
    - name: account
      image: quay.io/isanchez/grpc-demo-account
      version: v1.0.0
      replicas: 1
      limits:
        memory: 200Mi
        cpu: "0.5"
      requests:
        memory: 100Mi
        cpu: "0.1"
    - name: order
      image: quay.io/isanchez/grpc-demo-order
      version: v1.0.0
      replicas: 1
      limits:
        memory: 200Mi
        cpu: "0.5"
      requests:
        memory: 100Mi
        cpu: "0.1"
    - name: product
      image: quay.io/isanchez/grpc-demo-product
      version: v1.0.0
      replicas: 1
      limits:
        memory: 200Mi
        cpu: "0.5"
      requests:
        memory: 100Mi
        cpu: "0.1"
    - name: user
      image: quay.io/isanchez/grpc-demo-user
      version: v1.0.0
      replicas: 1
      limits:
        memory: 200Mi
        cpu: "0.5"
      requests:
        memory: 100Mi
        cpu: "0.1"
```

Create the custom resource in your cluster;

```bash
$ kubectl apply -f config/samples/grpcdemo_v1_demoservices.yaml
demoservices.grpcdemo.example.com/example-services created
```

After a few minutes the operator should have created all the required objects and the services should be up an running:

```bash
$ kubectl get deployment
NAME                                    READY   UP-TO-DATE   AVAILABLE   AGE
account                                 1/1     1            1           2m14s
grpc-demo-operator-controller-manager   1/1     1            1           14m
order                                   1/1     1            1           2m14s
product                                 1/1     1            1           2m13s
user                                    1/1     1            1           2m13s
```
