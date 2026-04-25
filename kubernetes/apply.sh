#!/bin/bash
set -e

echo "Creating namespace..."
kubectl apply -f namespace.yaml

echo "Creating secrets..."
source /workspace/.env.secrets
envsubst < secrets.yaml | kubectl apply -f -

echo "Creating configmap..."
kubectl apply -f configmap.yaml

echo "Deploying postgres..."
kubectl apply -f postgres/pvc.yaml
kubectl apply -f postgres/deployment.yaml
kubectl apply -f postgres/service.yaml

echo "Deploying redis..."
kubectl apply -f redis/pvc.yaml
kubectl apply -f redis/deployment.yaml
kubectl apply -f redis/service.yaml

echo "Deploying zookeeper..."
kubectl apply -f zookeeper/deployment.yaml
kubectl apply -f zookeeper/service.yaml

echo "Deploying kafka..."
kubectl apply -f kafka/deployment.yaml
kubectl apply -f kafka/service.yaml

echo "Creating logs PVC..."
kubectl apply -f logs/pvc.yaml

echo "Waiting for postgres..."
kubectl wait --namespace auth \
  --for=condition=ready pod \
  --selector=app=postgres \
  --timeout=90s

echo "Waiting for kafka..."
kubectl wait --namespace auth \
  --for=condition=ready pod \
  --selector=app=kafka \
  --timeout=90s

echo "Deploying auth-service..."
kubectl apply -f auth-service/deployment.yaml
kubectl apply -f auth-service/service.yaml

echo "Done!"
kubectl get all -n auth