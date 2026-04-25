#!/bin/bash
set -e

echo "Creating namespace..."
kubectl apply -f kubernetes/base/namespace.yaml

echo "Creating secrets..."
source .env.secrets
envsubst < kubernetes/base/secrets.yaml | kubectl apply -f -

echo "Creating configmap..."
kubectl apply -f kubernetes/base/configmap.yaml

echo "Deploying postgres..."
kubectl apply -f kubernetes/postgres/pvc.yaml
kubectl apply -f kubernetes/postgres/deployment.yaml
kubectl apply -f kubernetes/postgres/service.yaml

echo "Deploying redis..."
kubectl apply -f kubernetes/redis/pvc.yaml
kubectl apply -f kubernetes/redis/deployment.yaml
kubectl apply -f kubernetes/redis/service.yaml

echo "Deploying zookeeper..."
kubectl apply -f kubernetes/zookeeper/deployment.yaml
kubectl apply -f kubernetes/zookeeper/service.yaml

echo "Deploying kafka..."
kubectl apply -f kubernetes/kafka/deployment.yaml
kubectl apply -f kubernetes/kafka/service.yaml

echo "Creating logs PVC..."
kubectl apply -f kubernetes/logs/pvc.yaml

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
kubectl apply -f kubernetes/auth-service/deployment.yaml
kubectl apply -f kubernetes/auth-service/service.yaml

echo "Done!"
kubectl get all -n auth