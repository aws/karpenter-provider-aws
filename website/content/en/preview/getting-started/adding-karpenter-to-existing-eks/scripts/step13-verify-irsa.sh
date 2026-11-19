aws iam get-role --role-name "KarpenterController-${CLUSTER_NAME}" \
  --query 'Role.{arn:Arn,trust:AssumeRolePolicyDocument.Statement[0].Principal.Federated}'
aws iam list-attached-role-policies --role-name "KarpenterController-${CLUSTER_NAME}" \
  --query 'AttachedPolicies[].PolicyName'
