# Please forgive me for this -- build AWS SDK v2 from source, see https://github.com/aws/aws-cli/issues/4685#issuecomment-1094307056
FROM python:3.9-alpine as installer

RUN set -ex; \
    apk add --no-cache \
    git unzip groff \
    build-base libffi-dev cmake

ENV AWS_CLI_VERSION=2.7.1
RUN set -eux; \
    mkdir /aws; \
    git clone --single-branch --depth 1 -b ${AWS_CLI_VERSION} https://github.com/aws/aws-cli.git /aws; \
    cd /aws; \
    sed -i'' 's/PyInstaller.*/PyInstaller==4.10/g' requirements-build.txt; \
    python -m venv venv; \
    . venv/bin/activate; \
    ./scripts/installers/make-exe

RUN set -ex; \
    unzip /aws/dist/awscli-exe.zip; \
    ./aws/install --bin-dir /aws-cli-bin; \
    /aws-cli-bin/aws --version

FROM golang:1.19-alpine

# Install APK packages
RUN echo "@testing http://dl-cdn.alpinelinux.org/alpine/edge/testing" >>/etc/apk/repositories
RUN apk update
RUN apk add --no-cache \
    curl \
    tar \
    bash \
    gcc \
    alpine-sdk \
    git \
    jq \
    make \
    gcompat \
    helm@testing \
    kubectl@testing

# Install AWS CLI
COPY --from=installer /usr/local/aws-cli/ /usr/local/aws-cli/
COPY --from=installer /aws-cli-bin/ /usr/local/bin/
RUN aws --version

# Build release-notification-listener
RUN mkdir /release-notification-listener
COPY go.mod /release-notification-listener
COPY go.sum /release-notification-listener
COPY main.go /release-notification-listener
COPY listener /release-notification-listener/listener

WORKDIR /release-notification-listener

RUN GOPROXY=direct go build .

# Install tekton CLI
RUN curl --silent --location "https://github.com/tektoncd/cli/releases/download/v0.28.0/tkn_0.28.0_$(uname -s)_x86_64.tar.gz" | tar xz -C /tmp
RUN mv /tmp/tkn /usr/local/bin

RUN go build -o release-notification-listener

ENTRYPOINT [ "/release-notification-listener/release-notification-listener" ]
