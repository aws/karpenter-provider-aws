KARPENTER_NAMESPACE ?= kube-system
KO_DOCKER_REPO ?= ${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_DEFAULT_REGION}.amazonaws.com/kwok-karpenter
CPU_RESOURCES ?= "2"
MEMORY_RESOURCES ?= "4Gi"

## Extra helm options
CLUSTER_NAME ?= $(shell kubectl config view --minify -o jsonpath='{.clusters[].name}' | rev | cut -d"/" -f1 | rev | cut -d"." -f1)
CLUSTER_ENDPOINT ?= $(shell kubectl config view --minify -o jsonpath='{.clusters[].cluster.server}')
AWS_ACCOUNT_ID ?= $(shell aws sts get-caller-identity --query Account --output text)
KARPENTER_IAM_ROLE_ARN ?= arn:aws:iam::${AWS_ACCOUNT_ID}:role/${CLUSTER_NAME}-karpenter
HELM_OPTS ?= --set serviceAccount.annotations.eks\\.amazonaws\\.com/role-arn=${KARPENTER_IAM_ROLE_ARN} \
      		--set settings.clusterName=${CLUSTER_NAME} \
			--set settings.interruptionQueue=${CLUSTER_NAME} \
			--set controller.resources.requests.cpu=${CPU_RESOURCES} \
			--set controller.resources.requests.memory=${MEMORY_RESOURCES} \
			--set controller.resources.limits.cpu=${CPU_RESOURCES} \
			--set controller.resources.limits.memory=${MEMORY_RESOURCES} \
			--set settings.featureGates.nodeRepair=true \
			--set settings.featureGates.reservedCapacity=true \
			--set settings.featureGates.spotToSpotConsolidation=true \
			--set settings.preferencePolicy=Ignore \
			--create-namespace

KARPENTER_CORE_DIR = $(shell go list -m -f '{{ .Dir }}' sigs.k8s.io/karpenter)

.PHONY: image
image: ## Build the KWOK controller images using ko build
	$(eval CONTROLLER_IMG=$(shell KOCACHE=$(KOCACHE) KO_DOCKER_REPO="$(KO_DOCKER_REPO)" ko build --bare github.com/aws/karpenter-provider-aws/kwok))
	$(eval IMG_REPOSITORY=$(shell echo $(CONTROLLER_IMG) | cut -d "@" -f 1 | cut -d ":" -f 1))
	$(eval IMG_TAG=$(shell echo $(CONTROLLER_IMG) | cut -d "@" -f 1 | cut -d ":" -f 2 -s))
	$(eval IMG_DIGEST=$(shell echo $(CONTROLLER_IMG) | cut -d "@" -f 2))

.PHONY: apply
apply: image
	kubectl apply -f ../pkg/apis/crds/
	KARPENTER_NAMESPACE=$(KARPENTER_NAMESPACE) envsubst < ./manifests/clusterrole.yaml | kubectl apply -f -
	helm upgrade --install kwok-karpenter ../charts/karpenter --namespace ${KARPENTER_NAMESPACE} \
        $(HELM_OPTS) \
        --set logLevel=debug \
        --set controller.image.repository=$(IMG_REPOSITORY) \
        --set controller.image.tag=$(IMG_TAG) \
        --set controller.image.digest=$(IMG_DIGEST) \
        --set-string controller.env[0].name=ENABLE_PROFILING \
        --set-string controller.env[0].value=true

.PHONY: delete
delete:
	helm uninstall karpenter --namespace ${KARPENTER_NAMESPACE}

.PHONY: apply-kwok
apply-kwok:
	chmod +x $(KARPENTER_CORE_DIR)/hack/install-kwok.sh
	UNINSTALL_KWOK=false $(KARPENTER_CORE_DIR)/hack/install-kwok.sh

.PHONY: delete-kwok
delete-kwok:
	chmod +x $(KARPENTER_CORE_DIR)/hack/install-kwok.sh
	UNINSTALL_KWOK=true $(KARPENTER_CORE_DIR)/hack/install-kwok.sh

