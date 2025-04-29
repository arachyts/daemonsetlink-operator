# DaemonSetLink Operator
This operator links DaemonSets to Deployments or StatefulSets. When the source object is scaled to 0, the DaemonSet is scaled to 0 too
## Description
The operator scales a DS to 0 by applying a non-existent node selector. Usually DS should run on all (selected) nodes in a cluster, however there are cases where scaling DS down to 0 might be desirable. For example, if a log collector is scaled down, there's little purpose to run log agents on all nodes. Tools like KEDA do not support scaling DS because DS doesn't have scaling API.

Real world example: scale promtail DS down to 0 if loki is scaled down to 0.

## Getting Started TL;DR - I only want to install it in my cluster

```sh
kubectl apply -f https://raw.githubusercontent.com/arachyts/daemonsetlink-operator/v0.1.3/dist/install.yaml
```

This will install CRDs, permissions, and create a deployment in `kube-system` namespace.

You can then configure `DaemonSetLink` objects to start tracking specific source-target pairs. See examples in [samples](https://github.com/arachyts/daemonsetlink-operator/blob/main/config/samples/operators_v1alpha1_daemonsetlink.yaml)

## Getting Started

### Prerequisites
- go version v1.22.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-buildx IMG=<some-registry>/daemonsetlink-operator:tag PLATFORMS="linux/amd64,linux/arm64"
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/daemonsetlink-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/operators_v1alpha1_daemonsetlink.yaml
```

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/operators_v1alpha1_daemonsetlink.yaml
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

