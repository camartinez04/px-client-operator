# px-client-operator

This Operator will help you to deploy the [Portworx Client Application](https://github.com/camartinez04/portworx-client)

## Description

Portworx Client Application is a simple application which enables a web interface to interact with the Portworx gRPC API.

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

It is required to have a Portworx cluster running in the Kubernetes cluster.

### Running on the cluster

1. Build and push your image to the location specified by `IMG`, not needed if you only want to use this operator:
	
```sh
make docker-build docker-push IMG=calvarado04.com/px-client-operator:latest
```
	
2. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy
```
This will install the CRDs and deploy the controller to the cluster on px-client namespace.

3. If cluster is OpenShift/OKD. Add anyuid SCC to the default service account in the namespace where the operator is deployed (px-client).
```sh
oc adm policy add-scc-to-user anyuid -z default -n px-client
```

4. If you have Portworx with Security enabled. Add the Token to portworxToken spec on `./config/samples/pxclient_v1alpha1_broker.yaml` from secret `px-admin-token` present in the namespace where Portworx was installed.

5. Install Instances of these Custom Resource Definitions (CRDs) on the cluster:

```sh

```sh
kubectl apply -f ./config/samples/
```


### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing
Feel free to download and contribute to this project.

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/) 
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster 

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

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

