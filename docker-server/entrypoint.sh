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

# Deploy if project exists
if [ -f /workspace/Makefile ]; then
    echo "▶ Deploying auth-service..."
    cd /workspace

    # Generate secrets if .env.secrets doesn't exist
    if [ ! -f .env.secrets ]; then
        echo "▶ Generating secrets..."
        cat > .env.secrets << EOF
AUTH_DB_PASSWORD=postgres
AUTH_JWT_ACCESS_SECRET=$(openssl rand -hex 32)
AUTH_JWT_REFRESH_SECRET=$(openssl rand -hex 32)
AUTH_JWT_CANONICAL_SECRET=$(openssl rand -hex 32)
EOF
    fi

    make k8s-apply

    echo "▶ Waiting for auth-service to be ready..."
    kubectl wait --namespace auth \
        --for=condition=ready pod \
        --selector=app=auth-service \
        --timeout=180s || true
fi

# ── Forward ports to container's public interface ─────────────────────
MINIKUBE_IP=$(minikube ip)
echo "▶ Minikube IP: $MINIKUBE_IP"
echo "▶ Starting port forwards via socat..."

nohup socat TCP-LISTEN:8080,fork,reuseaddr TCP:$MINIKUBE_IP:30080 > /tmp/socat-http.log 2>&1 &
nohup socat TCP-LISTEN:50051,fork,reuseaddr TCP:$MINIKUBE_IP:30051 > /tmp/socat-grpc.log 2>&1 &

echo "✅ Server ready!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  SSH:  ssh root@localhost -p 2222  (password: root)"
echo "  HTTP: http://localhost:8080"
echo "  gRPC: localhost:50051"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Keep container alive
tail -f /dev/null