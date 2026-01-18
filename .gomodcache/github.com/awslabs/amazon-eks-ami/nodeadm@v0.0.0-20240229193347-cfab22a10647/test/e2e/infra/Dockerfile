FROM golang:1.21 AS imds-mock-build
RUN go env -w GOPROXY=direct
RUN GOBIN=/bin go install github.com/aws/amazon-ec2-metadata-mock/cmd@v1.11.2
RUN mv /bin/cmd /imds-mock

FROM golang:1.21 AS nodeadm-build
WORKDIR /go/src/github.com/awslabs/amazon-eks-ami/nodeadm
COPY . .
RUN make build
RUN mv _bin/nodeadm /nodeadm

FROM public.ecr.aws/amazonlinux/amazonlinux:2023
RUN dnf -y update && \
    dnf -y install systemd containerd jq && \
    dnf clean all
COPY --from=imds-mock-build /imds-mock /usr/local/bin/imds-mock
COPY test/e2e/infra/aemm-default-config.json /etc/aemm-default-config.json
COPY --from=nodeadm-build /nodeadm /usr/local/bin/nodeadm
COPY test/e2e/infra/systemd/kubelet.service /usr/lib/systemd/system/kubelet.service
COPY test/e2e/infra/systemd/containerd.service /usr/lib/systemd/system/containerd.service
COPY test/e2e/infra/mock/ /sys_devices_system_mock/
COPY test/e2e/helpers.sh /helpers.sh

RUN mkdir -p /etc/eks/image-credential-provider/
RUN touch /etc/eks/image-credential-provider/ecr-credential-provider
ENV CPU_DIR /sys_devices_system_mock/cpu
ENV NODE_DIR /sys_devices_system_mock/node

ENTRYPOINT ["/usr/lib/systemd/systemd","--system"]
