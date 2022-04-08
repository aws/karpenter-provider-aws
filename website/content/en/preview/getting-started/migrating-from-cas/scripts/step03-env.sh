CLUSTER_NAME=<your existing cluster>
OIDC_ENDPOINT=$(aws eks describe-cluster --name ${CLUSTER_NAME} \
    --query "cluster.identity.oidc.issuer" --output text)
ACCOUNT_NUMBER=$(aws sts get-caller-identity --query 'Account' \
    --output text)
