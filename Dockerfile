FROM golang:1.14.4 as builder

# Copy src
WORKDIR /go/src/github.com/ellistarn/karpenter
COPY cmd/    cmd/
COPY pkg/    pkg/
COPY go.mod  go.mod
COPY go.sum  go.sum

# Build src
RUN GOPROXY=direct go mod download
RUN go build -o karpenter ./cmd

# Copy to slim image
FROM gcr.io/distroless/base:latest
WORKDIR /
COPY --from=builder /go/src/github.com/ellistarn/karpenter .
ENTRYPOINT ["/karpenter"]
