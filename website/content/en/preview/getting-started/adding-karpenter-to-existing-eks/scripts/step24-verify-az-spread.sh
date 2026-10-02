kubectl get nodes -l <your-system-label-key>=<your-system-label-value> \
  -o jsonpath='{.items[*].metadata.labels.topology\.kubernetes\.io/zone}' \
  | tr ' ' '\n' | sort -u | wc -l
