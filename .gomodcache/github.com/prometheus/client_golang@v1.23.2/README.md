# Prometheus Go client library

[![CI](https://github.com/prometheus/client_golang/actions/workflows/go.yml/badge.svg)](https://github.com/prometheus/client_golang/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/prometheus/client_golang)](https://goreportcard.com/report/github.com/prometheus/client_golang)
[![Go Reference](https://pkg.go.dev/badge/github.com/prometheus/client_golang.svg)](https://pkg.go.dev/github.com/prometheus/client_golang)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/prometheus/client_golang/badge)](https://securityscorecards.dev/viewer/?uri=github.com/prometheus/client_golang)
[![Slack](https://img.shields.io/badge/join%20slack-%23prometheus--client_golang-brightgreen.svg)](https://slack.cncf.io/)

This is the [Go](http://golang.org) client library for
[Prometheus](http://prometheus.io). It has two separate parts, one for
instrumenting application code, and one for creating clients that talk to the
Prometheus HTTP API.

## Version Compatibility

This library supports the two most recent major releases of Go. While it may function with older versions, we only provide fixes and support for the currently supported Go releases.

> [!NOTE]
> See our [Release Process](RELEASE.md#supported-go-versions) for details on compatibility and support policies.

## Important note about releases and stability

This repository generally follows [Semantic
Versioning](https://semver.org/). However, the API client in
`prometheus/client_golang/api/…` is still considered experimental. Breaking
changes of the API client will _not_ trigger a new major release. The same is
true for selected other new features explicitly marked as **EXPERIMENTAL** in
CHANGELOG.md.

Features that require breaking changes in the stable parts of the repository
are being batched up and tracked in the [v2
milestone](https://github.com/prometheus/client_golang/milestone/2), but plans for further development of v2 at the moment.

> NOTE: The initial v2 attempt is in a [separate branch](https://github.com/prometheus/client_golang/tree/dev-v2). We also started
experimenting on a new `prometheus.V2.*` APIs in [the 1.x's V2 struct](https://github.com/prometheus/client_golang/blob/main/prometheus/vnext.go#L23). Help wanted!

## Instrumenting applications

[![Go Reference](https://pkg.go.dev/badge/github.com/prometheus/client_golang/prometheus.svg)](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus)

The
[`prometheus` directory](https://github.com/prometheus/client_golang/tree/main/prometheus)
contains the instrumentation library. See the
[guide](https://prometheus.io/docs/guides/go-application/) on the Prometheus
website to learn more about instrumenting applications.

The
[`examples` directory](https://github.com/prometheus/client_golang/tree/main/examples)
contains simple examples of instrumented code.

## Client for the Prometheus HTTP API

[![Go Reference](https://pkg.go.dev/badge/github.com/prometheus/client_golang/api.svg)](https://pkg.go.dev/github.com/prometheus/client_golang/api)

The
[`api/prometheus` directory](https://github.com/prometheus/client_golang/tree/main/api/prometheus)
contains the client for the
[Prometheus HTTP API](http://prometheus.io/docs/querying/api/). It allows you
to write Go applications that query time series data from a Prometheus
server. It is still in alpha stage.

## Where is `model`, `extraction`, and `text`?

The `model` packages has been moved to
[`prometheus/common/model`](https://github.com/prometheus/common/tree/main/model).

The `extraction` and `text` packages are now contained in
[`prometheus/common/expfmt`](https://github.com/prometheus/common/tree/main/expfmt).

## Contributing and community

See the [contributing guidelines](CONTRIBUTING.md) and the
[Community section](http://prometheus.io/community/) of the homepage.

`client_golang` community is also present on the CNCF Slack `#prometheus-client_golang`.
