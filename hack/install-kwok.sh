# get teh latest version
KWOK_WORK_DIR=$(mktemp -d)
KWOK_REPO=kubernetes-sigs/kwok
KWOK_LATEST_RELEASE=$(curl "https://api.github.com/repos/${KWOK_REPO}/releases/latest" | jq -r '.tag_name')


# allow it to tolerate everything, but not run on nodes that we launch
cat <<EOF > "${KWOK_WORK_DIR}/tolerate-all.yaml"
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kwok-controller
  namespace: kube-system
spec:
  template:
    spec:
      tolerations:
      - operator: "Exists"
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kwok.x-k8s.io/node
                operator: DoesNotExist
EOF

cat <<EOF > "${KWOK_WORK_DIR}/kustomization.yaml"
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
images:
- name: registry.k8s.io/kwok/kwok
  newTag: "${KWOK_LATEST_RELEASE}"
resources:
- "https://github.com/${KWOK_REPO}/kustomize/kwok?ref=${KWOK_LATEST_RELEASE}"
patchesStrategicMerge:
- tolerate-all.yaml
EOF

kubectl kustomize "${KWOK_WORK_DIR}" > "${KWOK_WORK_DIR}/kwok.yaml"

kubectl apply -f "${KWOK_WORK_DIR}/kwok.yaml"
