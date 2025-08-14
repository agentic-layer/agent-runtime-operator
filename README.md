# Agent Runtime Operator

A Kubernetes operator for the agentic layer agent runtime component.
Responsible to deploy and manage specific `Agent` instances.

The basic idea is to provide a framework agnostic abstraction in the form of 
a CRD to deploy and manage agentic workloads. The abstraction takes care 
of framework specific configuration and wiring of all essentiell cross-cutting 
components, like the model router.

```yaml
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  labels:
    app.kubernetes.io/name: agent-runtime-operator
    app.kubernetes.io/managed-by: kustomize
  name: weather-agent
spec:
  framework: google-adk
  image: europe-west3-docker.pkg.dev/qaware-paal/agentic-layer/weather-agent:v0.1.0
  protocols:
    - type: A2A
  replicas: 1
```

## Development

This Kubernetes operator is built using the [Kubebuilder](https://book.kubebuilder.io/) framework.

Build the operator using the following command:

```shell
make
```

Install the CRDs into your current Kubernetes cluster:

```shell
make install
```

Prepare the environment for running the operator:

```shell
# Ensure you have a local Kubernetes cluster running.
kind create cluster

# Install cert-manager for webhook support
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.18.2/cert-manager.yaml

# Build the operator image and push it to the local registry
make docker-build
make kind-load

# Apply the operator manifests to your current Kubernetes cluster
make deploy
```

Run the operator locally against your current Kubernetes cluster:

```shell
# Run only the controller locally.
# Note: If you deployed everything using `make deploy`, a controller is already running and may interfere with this.
# See also: https://book.kubebuilder.io/cronjob-tutorial/running
export ENABLE_WEBHOOKS=false
make run
```

## License

This software is provided under the Apache v2.0 open source license, read the `LICENSE` file for details.
