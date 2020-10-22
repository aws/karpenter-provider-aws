FROM golang:1.15.3 as builder

# Copy src
WORKDIR /go/src/github.com/ellistarn/karpenter
COPY go.mod  go.mod
COPY go.sum  go.sum

# Build src
RUN GOPROXY=direct go mod download

COPY karpenter/ karpenter/
COPY pkg/ pkg/

RUN go build -o karpenter ./karpenter

# Copy to slim image
FROM gcr.io/distroless/base:latest
WORKDIR /
COPY --from=builder /go/src/github.com/ellistarn/karpenter .
ENTRYPOINT ["/karpenter"]
