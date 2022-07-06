echo "Installing Tekton"

kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.33.2/release.yaml
kubectl patch configmap config-defaults -n tekton-pipelines --patch '{"data": { "default-task-run-workspace-binding": "emptyDir: {}" } }'
kubectl patch deployment tekton-pipelines-controller -n tekton-pipelines --patch '{"spec":{"template":{"spec":{"tolerations":[{"key":"CriticalAddonsOnly", "operator":"Exists"}]}}}}'
kubectl patch deployment tekton-pipelines-webhook -n tekton-pipelines --patch '{"spec":{"template":{"spec":{"tolerations":[{"key":"CriticalAddonsOnly", "operator":"Exists"}]}}}}'
sleep 10

kubectl apply -f https://storage.googleapis.com/tekton-releases/triggers/previous/v0.19.0/release.yaml
kubectl patch deployment tekton-triggers-controller -n tekton-pipelines --patch '{"spec":{"template":{"spec":{"tolerations":[{"key":"CriticalAddonsOnly", "operator":"Exists"}]}}}}'
kubectl patch deployment tekton-triggers-webhook -n tekton-pipelines --patch '{"spec":{"template":{"spec":{"tolerations":[{"key":"CriticalAddonsOnly", "operator":"Exists"}]}}}}'
sleep 10

kubectl apply -f https://github.com/tektoncd/dashboard/releases/download/v0.24.1/tekton-dashboard-release.yaml
kubectl patch deployment tekton-dashboard -n tekton-pipelines --patch '{"spec":{"template":{"spec":{"tolerations":[{"key":"CriticalAddonsOnly", "operator":"Exists"}]}}}}'
sleep 10
