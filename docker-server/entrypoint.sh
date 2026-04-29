#!/bin/bash
set -e

echo "▶ Starting SSH server..."
service ssh start

echo "▶ Starting Docker daemon..."
dockerd > /tmp/dockerd.log 2>&1 &
DOCKERD_PID=$!

echo "▶ Waiting for Docker..."
until docker info > /dev/null 2>&1; do
    sleep 1
done
echo "✅ Docker ready"

echo "▶ Starting Minikube..."
minikube start \
    --driver=docker \
    --force \
    --cpus=4 \
    --memory=4096 \
    --addons=ingress

echo "✅ Minikube ready"

# Deploy using apply.sh
if [ -f /workspace/kubernetes/apply.sh ]; then
    echo "▶ Running apply.sh..."
    cd /workspace/kubernetes
    chmod +x apply.sh
    ./apply.sh

    echo "▶ Waiting for auth-service..."
    kubectl wait --namespace auth \
        --for=condition=ready pod \
        --selector=app=auth-service \
        --timeout=180s || true

    echo "▶ Waiting for payment-gateway..."
    kubectl wait --namespace auth \
        --for=condition=ready pod \
        --selector=app=payment-gateway \
        --timeout=180s || true
fi

# ── Forward ports to container's public interface ─────────────────────
MINIKUBE_IP=$(minikube ip)
echo "▶ Minikube IP: $MINIKUBE_IP"
echo "▶ Starting port forwards via socat..."

# Auth service
nohup socat TCP-LISTEN:8080,fork,reuseaddr TCP:$MINIKUBE_IP:30080 > /tmp/socat-auth-http.log 2>&1 &
nohup socat TCP-LISTEN:50051,fork,reuseaddr TCP:$MINIKUBE_IP:30051 > /tmp/socat-auth-grpc.log 2>&1 &

# Payment gateway
nohup socat TCP-LISTEN:8081,fork,reuseaddr TCP:$MINIKUBE_IP:30081 > /tmp/socat-payment-http.log 2>&1 &
nohup socat TCP-LISTEN:50052,fork,reuseaddr TCP:$MINIKUBE_IP:30052 > /tmp/socat-payment-grpc.log 2>&1 &

# Ingress
nohup socat TCP-LISTEN:80,fork,reuseaddr TCP:$MINIKUBE_IP:80 > /tmp/socat-ingress.log 2>&1 &

echo "✅ Server ready!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  SSH:  ssh root@localhost -p 2222  (password: root)"
echo "  AUTH HTTP:     http://localhost:8080"
echo "  AUTH gRPC:     localhost:50051"
echo "  PAYMENT HTTP:  http://localhost:8081"
echo "  PAYMENT gRPC:  localhost:50052"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Keep container alive
tail -f /dev/null