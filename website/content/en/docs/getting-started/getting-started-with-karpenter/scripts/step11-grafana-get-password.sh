kubectl get secret --namespace monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode
