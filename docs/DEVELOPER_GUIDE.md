# Contributing

## Dependencies

The following tools are required for doing development on Karpenter.

| Package                                                                     | Version  | Install                                                                 |
| --------------------------------------------------------------------------- | -------- | ----------------------------------------------------------------------- |
| [go](https://golang.org/dl/)                                                | v1.14.4+ |                                                                         |
| [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/)          |          |                                                                         |
| [helm](https://helm.sh/docs/intro/install/)                                 |          | `brew install helm`                                                     |
| Other tools                                                                 |          | `make toolchain`                                                        |

## Developing

### Setup / Teardown

```
./hack/quick-install.sh
./hack/quick-install.sh --delete
```

### Developer Loop

Local development is not supported at this time.

Tips:

* Make sure dependencies are installed
* You will need a personal development image repository (e.g. ECR)
* $KO_DOCKER_REPO must point to your development repository
* Your cluster must have permissions to read from the repository

Workflow:

1. Edit files locally
2. Test changes: `make test`
3. Apply changes: `make deploy`

### Testing

TODO

## Debugging Prometheus

Open up the UI:

```
open http://localhost:8080 && kubectl port-forward service/prometheus-server -n prometheus 8080:80
```
