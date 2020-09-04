# Contributing
## Dependencies

The following tools are required for development.

1. [go v1.14.4+](https://golang.org/dl/)
2. [docker](https://docs.docker.com/install/)
3. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
4. [kustomize](https://kubernetes-sigs.github.io/kustomize/installation/)

   1. OSX: `brew install kustomize`
5. [controller-gen](https://book.kubebuilder.io/reference/controller-gen.html)

   1. `go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5`
6. [helm](https://helm.sh/docs/intro/install/)

   1. OSX: `brew install helm`

## Developing
### Setup / Teardown
```
./hack/quick-install.sh
./hack/quick-install.sh --delete
```

### Developer Loop
Local development is not supported at this time.

Tips:
- Make sure dependencies are installed
- You will need a personal development image repository (e.g. ECR)
- $KO_DOCKER_REPO must point to your development repository
- Your cluster must have permissions to read from the repository

Workflow:
1. Edit files locally
2. Test changes: `make test`
3. Apply changes: `make deploy`


## Debugging Prometheus
Open up the UI:
```
open http://localhost:8080 && kubectl port-forward service/prometheus-server -n prometheus 8080:80
```
