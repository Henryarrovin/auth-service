#!/bin/bash
set -e

echo "Creating namespace..."
kubectl apply -f k8s/base/namespace.yaml

echo "Creating secrets..."
source .env.secrets
envsubst < k8s/base/secrets.yaml | kubectl apply -f -

echo "Creating configmap..."
kubectl apply -f k8s/base/configmap.yaml

echo "Deploying postgres..."
kubectl apply -f k8s/postgres/pvc.yaml
kubectl apply -f k8s/postgres/deployment.yaml
kubectl apply -f k8s/postgres/service.yaml

echo "Deploying redis..."
kubectl apply -f k8s/redis/pvc.yaml
kubectl apply -f k8s/redis/deployment.yaml
kubectl apply -f k8s/redis/service.yaml

echo "Deploying zookeeper..."
kubectl apply -f k8s/zookeeper/deployment.yaml
kubectl apply -f k8s/zookeeper/service.yaml

echo "Deploying kafka..."
kubectl apply -f k8s/kafka/deployment.yaml
kubectl apply -f k8s/kafka/service.yaml

echo "Creating logs PVC..."
kubectl apply -f k8s/logs/pvc.yaml

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
kubectl apply -f k8s/auth-service/deployment.yaml
kubectl apply -f k8s/auth-service/service.yaml

echo "Done!"
kubectl get all -n auth