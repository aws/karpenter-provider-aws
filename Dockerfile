FROM golang:1.14.4 as builder

# Copy src
WORKDIR /go/src/github.com/aws/karpenter
COPY cmd/    cmd/
COPY pkg/    pkg/
COPY go.mod  go.mod
COPY go.sum  go.sum

# Build src
RUN go mod download
RUN go build -o karpenter ./cmd/karpenter

# Copy to slim image
FROM gcr.io/distroless/static:latest
WORKDIR /
COPY --from=builder /go/src/github.com/aws/karpenter .
ENTRYPOINT ["/karpenter"]