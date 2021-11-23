
---
title: "Monitor Karpenter with Grafana Dashboards"
linkTitle: "Grafana Dashboards"
weight: 10
---

The Karpenter repo contains multiple [importable dashboards](https://github.com/awslabs/karpenter/tree/main/grafana-dashboards) for an existing Grafana instance. See the Grafana documentation for [instructions](https://grafana.com/docs/grafana/latest/dashboards/export-import/#import-dashboard) to import a dashboard.

#### Deploy a temporary Prometheus and Grafana stack (optional)

The following commands will deploy a Prometheus and Grafana stack that is suitable for this guide but does not include persistent storage or other configurations that would be necessary for monitoring a production deployment of Karpenter.

```sh
helm repo add grafana-charts https://grafana.github.io/helm-charts
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

kubectl create namespace monitoring

curl -fsSL https://karpenter.sh/docs/getting-started/prometheus-values.yaml
helm install --namespace monitoring prometheus prometheus-community/prometheus --values prometheus-values.yaml

curl -fsSL https://karpenter.sh/docs/getting-started/grafana-values.yaml
helm install --namespace monitoring grafana grafana-charts/grafana --values grafana-values.yaml
```

The Grafana instance may be accessed using port forwarding.

```sh
kubectl port-forward --namespace monitoring svc/grafana 3000:80
```

The new stack has only one user, `admin`, and the password is stored in a secret. The following command will retrieve the password.

```sh
kubectl get secret --namespace monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode
```