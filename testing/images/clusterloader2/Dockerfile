FROM golang:1.16.4 AS builder
WORKDIR /go/src/k8s.io
RUN git clone https://github.com/kubernetes/perf-tests
WORKDIR /go/src/k8s.io/perf-tests/clusterloader2
RUN GOPROXY=direct GOOS=linux CGO_ENABLED=0  go build -o ./clusterloader ./cmd

FROM alpine:3.15.4
WORKDIR /
COPY --from=builder /go/src/k8s.io/perf-tests/clusterloader2/clusterloader /clusterloader
ENTRYPOINT ["/clusterloader"]
