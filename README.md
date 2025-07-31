# Agent Runtime Operator

A Kubernetes operator for the agentic layer agent runtime component.
Responsible to deploy and manage specific `Agent` instances.

The basic idea is to provide a framework agnostic abstraction in the form of 
a CRD to deploy and manage agentic workloads. The abstraction takes care 
of framework specific configuration and wiring of all essentiell corss-cutting 
components, like the model router.

```yaml
apiVersion: runtime.agentic-layer.ai/v1alpha1
kind: Agent
metadata:
  name: cross-selling-agent
  labels:
    domain: insurance
spec:
  framework: google-adk
  replicas: 2
  image: cross-selling-agent:latest
  ports:
    - name: http
      port: 8080
```

## Maintainer

M.-Leander Reimer (@lreimer), <mario-leander.reimer@qaware.de>

## License

This software is provided under the Apache v2.0 open source license, read the `LICENSE` file for details.

A Kubernetes operator for the agentic layer agent runtime component.
