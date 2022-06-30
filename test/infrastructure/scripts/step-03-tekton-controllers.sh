echo "Installing Tekton"
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.33.2/release.yaml
sleep 10
kubectl apply -f https://storage.googleapis.com/tekton-releases/triggers/previous/v0.19.0/release.yaml
sleep 10
kubectl apply -f https://github.com/tektoncd/dashboard/releases/download/v0.24.1/tekton-dashboard-release.yaml
sleep 10
