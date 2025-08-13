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

Run the operator locally against your current Kubernetes cluster:

```shell
make run
```

## License

This software is provided under the Apache v2.0 open source license, read the `LICENSE` file for details.
