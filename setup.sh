#!/bin/bash

set -xe

helm repo add jetstack https://charts.jetstack.io
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
helm repo update

# ceet-manager
helm upgrade --install \
  cert-manager jetstack/cert-manager \
  --namespace cert-manager \
  --create-namespace \
  --version v1.5.3 \
  --values cert-manager.values.yaml
kubectl apply -f cert-issuer.yaml

# nginx ingress
helm upgrade --install \
  ingress-nginx ingress-nginx/ingress-nginx \
  --values ingress-nginx.values.yaml \
  --namespace default
kubectl apply -f ingress.yaml

# kube metrics server
helm upgrade --install \
  metrics-server metrics-server/metrics-server \
  --namespace default

kubectl apply -R -f apps
