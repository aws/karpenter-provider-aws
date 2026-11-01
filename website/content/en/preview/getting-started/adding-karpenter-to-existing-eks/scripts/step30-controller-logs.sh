kubectl logs -n "${KARPENTER_NAMESPACE}" -l app.kubernetes.io/name=karpenter \
  -c controller --tail=200 -f \
  | grep -iE 'launched|registered|disrupt|consolidat|terminat'
