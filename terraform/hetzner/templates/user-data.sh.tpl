#!/bin/bash
set -e

echo "Starting Bootstrap Process for AI Platform..."

apt-get update && apt-get upgrade -y

curl -sfL https://get.k3s.io | INSTALL_K3S_VERSION="${k3s_version}" sh -

echo "Waiting for k3s to be ready..."
sleep 30
export KUBECONFIG=/etc/rancher/k3s/k3s.yaml

kubectl create namespace argocd
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/${argocd_version}/manifests/install.yaml

echo "Waiting for ArgoCD server..."
kubectl wait --for=condition=available deployment/argocd-server -n argocd --timeout=300s

kubectl apply -f https://raw.githubusercontent.com/Kirill140992/ai-inference-platform/main/argocd/root-app.yaml

echo "Bootstrap Complete! ArgoCD is now managing the cluster."