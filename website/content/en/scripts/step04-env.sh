CLUSTER_ENDPOINT="$(aws eks describe-cluster \
    --name ${CLUSTER_NAME} --query "cluster.endpoint" \
    --output text)"
OIDC_ENDPOINT="$(aws eks describe-cluster --name ${CLUSTER_NAME} \
    --query "cluster.identity.oidc.issuer" --output text)"
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query 'Account' \
    --output text)
