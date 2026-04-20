# Immortal Kubernetes Operator

CRDs + Helm chart that run Immortal as a Kubernetes-native controller. Operators declare policy via three CRDs: `Intent`, `Playbook`, `Incident`.

## What's here

```
operator/
  crds/             # three CustomResourceDefinition manifests
    intent.yaml
    playbook.yaml
    incident.yaml
  helm/             # Helm chart for deploying the engine as an operator
    Chart.yaml
    values.yaml
    templates/
      deployment.yaml
      configmap.yaml
      rbac.yaml
      service.yaml
      _helpers.tpl
```

## Install

```sh
# 1. Apply the CRDs (idempotent)
kubectl apply -f https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/operator/crds/

# 2. Install the chart
helm install immortal oci://ghcr.io/nagendhra-web/charts/immortal-operator --version 0.1.0

# Or from a local clone:
helm install immortal ./operator/helm -n immortal-system --create-namespace
```

## Declare an intent

```yaml
apiVersion: immortal.dev/v1
kind: Intent
metadata:
  name: protect-checkout
spec:
  description: Keep checkout + payments available at all costs.
  goals:
    - kind: LatencyUnder
      service: checkout
      target: 200
      priority: 10
    - kind: ErrorRateUnder
      service: checkout
      target: 0.005
      priority: 10
    - kind: ProtectService
      service: payments
      priority: 10
```

```sh
kubectl apply -f my-intent.yaml
kubectl get intents
kubectl describe intent protect-checkout
```

## Declare a playbook

```yaml
apiVersion: immortal.dev/v1
kind: Playbook
metadata:
  name: restart-rest-on-oom
spec:
  description: Restart the REST service when OOM events are observed.
  trigger:
    onIncidentKind: oom_kill
  steps:
    - name: snapshot
      action:
        kind: exec
        command: [kubectl, get, pod, -l, app=rest, -o, json]
    - name: restart
      action:
        kind: restart
        target: rest
      rollback:
        kind: exec
        command: [echo, noop]
```

## Status on incidents

Every incident the engine processes lands as an `Incident` CR with its Verdict:

```sh
kubectl get incidents -w
kubectl describe incident inc-42
```

The `Incident.status.verdict` includes `cause`, `evidence`, `action`, `outcome`, and `confidence` — the same Verdict structure the narrator produces in the dashboard.

## Roadmap

This release ships the CRDs, the Helm chart, and the engine image. The reconciler loop that translates `Intent` state changes into engine calls is implemented inline in the engine via the `/api/v6/intent` endpoint — a dedicated `kubebuilder`-style controller is tracked in [#31](https://github.com/Nagendhra-web/Immortal/issues/31) as a next iteration.

## Uninstall

```sh
helm uninstall immortal -n immortal-system
kubectl delete -f https://raw.githubusercontent.com/Nagendhra-web/Immortal/main/operator/crds/
kubectl delete namespace immortal-system
```
