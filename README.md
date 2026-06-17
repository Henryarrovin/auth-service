# auth-service

A production-ready authentication microservice exposing a gRPC API on port `50051` and an HTTP API on port `8080`. It supports JWT-based auth, OAuth (Google & GitHub), email verification and totp for 2FA.

---

## Quick Start

### 1. Pull the image

```bash
docker pull henryarrovin/auth-service
```

### 2. Build & run (include env -> important)

```bash
docker build -t auth-server .

docker run -d \
  --name auth-server \
  --privileged \
  -p 2222:22 \
  -p 8080:8080 \
  -p 50051:50051 \
  -v auth-server-docker:/var/lib/docker \
  -v auth-minikube:/root/.minikube \
  -v auth-kube:/root/.kube \
  auth-server
```

---

## Exposed Ports

| Port | Protocol | Description |
|------|----------|-------------|
| `22` | SSH | Remote access (mapped from `2222`) |
| `8080` | HTTP | REST API |
| `50051` | gRPC | gRPC API |

---

## Configuration

All configuration is provided via environment variables.

### Server

| Variable | Description | Default |
|----------|-------------|---------|
| `AUTH_SERVER_GRPC_PORT` | gRPC server port | `50051` |
| `AUTH_SERVER_ENV` | Runtime environment (`development` / `production`) | `development` |

### Database (PostgreSQL)

| Variable | Description | Default |
|----------|-------------|---------|
| `AUTH_DATABASE_HOST` | Database host | `localhost` |
| `AUTH_DATABASE_PORT` | Database port | `5432` |
| `AUTH_DATABASE_USER` | Database username | `postgres` |
| `AUTH_DATABASE_PASSWORD` | Database password | `postgres` |
| `AUTH_DATABASE_DBNAME` | Database name | `auth-db` |
| `AUTH_DATABASE_SSLMODE` | SSL mode (`disable` / `require`) | `disable` |

### Redis

| Variable | Description | Default |
|----------|-------------|---------|
| `AUTH_REDIS_ADDR` | Redis address (`host:port`) | `localhost:6379` |
| `AUTH_REDIS_PASSWORD` | Redis password | `""` |
| `AUTH_REDIS_DB` | Redis database index | `0` |

### Kafka (optional)

| Variable | Description | Default |
|----------|-------------|---------|
| `AUTH_KAFKA_ENABLED` | Enable Kafka logging | `true` |
| `AUTH_KAFKA_BROKERS` | Comma-separated broker list | `localhost:9092` |
| `AUTH_KAFKA_TOPIC` | Topic for auth service logs | `auth-service-logs` |
| `AUTH_KAFKA_GROUP_ID` | Consumer group ID | `auth-log-consumer` |
| `AUTH_KAFKA_LOG_DIR` | Local log directory | `/apps/logs` |

### Email (SMTP)

| Variable | Description |
|----------|-------------|
| `AUTH_EMAIL_HOST` | SMTP host (e.g. `smtp.gmail.com`) |
| `AUTH_EMAIL_PORT` | SMTP port (e.g. `587`) |
| `AUTH_EMAIL_USERNAME` | SMTP username |
| `AUTH_EMAIL_PASSWORD` | SMTP password |
| `AUTH_EMAIL_FROM` | Sender address |
| `AUTH_EMAIL_BASE_URL` | Base URL for email verification links |

### Google OAuth

Obtain credentials from the [Google Cloud Console](https://console.cloud.google.com/apis/credentials).

| Variable | Description |
|----------|-------------|
| `AUTH_OAUTH_GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `AUTH_OAUTH_GOOGLE_CLIENT_SECRET` | Google OAuth client secret |
| `AUTH_OAUTH_GOOGLE_REDIRECT_URL` | Authorized redirect URI |

### GitHub OAuth

Obtain credentials from [GitHub Developer Settings](https://github.com/settings/developers).

| Variable | Description |
|----------|-------------|
| `AUTH_OAUTH_GITHUB_CLIENT_ID` | GitHub OAuth app client ID |
| `AUTH_OAUTH_GITHUB_CLIENT_SECRET` | GitHub OAuth app client secret |
| `AUTH_OAUTH_GITHUB_REDIRECT_URL` | Authorized callback URL |

---

## Volumes

| Volume | Mount path | Description |
|--------|-----------|-------------|
| `auth-server-docker` | `/var/lib/docker` | Docker-in-Docker storage |
| `auth-minikube` | `/root/.minikube` | Minikube state |
| `auth-kube` | `/root/.kube` | Kubernetes config |

---

## License

MIT
