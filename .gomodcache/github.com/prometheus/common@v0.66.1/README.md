# Common
![circleci](https://circleci.com/gh/prometheus/common/tree/main.svg?style=shield)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/prometheus/common/badge)](https://securityscorecards.dev/viewer/?uri=github.com/prometheus/common)


This repository contains Go libraries that are shared across Prometheus
components and libraries. They are considered internal to Prometheus, without
any stability guarantees for external usage.

* **assets**: Embedding of static assets with gzip support
* **config**: Common configuration structures
* **expfmt**: Decoding and encoding for the exposition format
* **model**: Shared data structures
* **promslog**: A logging wrapper around [log/slog](https://pkg.go.dev/log/slog)
* **route**: A routing wrapper around [httprouter](https://github.com/julienschmidt/httprouter) using `context.Context`
* **server**: Common servers
* **version**: Version information and metrics
