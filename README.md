
docker pull henryarrovin/auth-service

// to run the server

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

# Env variables

## Server
AUTH_SERVER_GRPC_PORT=50051
AUTH_SERVER_ENV=development

## Database
AUTH_DATABASE_PORT=5432
AUTH_DATABASE_HOST=localhost
AUTH_DATABASE_USER=postgres
AUTH_DATABASE_PASSWORD=postgres
AUTH_DATABASE_DBNAME=auth-db
AUTH_DATABASE_SSLMODE=disable

## Redis
AUTH_REDIS_ADDR=localhost:6379
AUTH_REDIS_PASSWORD=""
AUTH_REDIS_DB=0

## kafka
AUTH_KAFKA_ENABLED=true
AUTH_KAFKA_TOPIC=auth-service-logs
AUTH_KAFKA_GROUP_ID=auth-log-consumer
AUTH_KAFKA_LOG_DIR=/apps/logs
AUTH_KAFKA_BROKERS=localhost:9092

## email
AUTH_EMAIL_HOST=smtp.gmail.com
AUTH_EMAIL_PORT=587
AUTH_EMAIL_USERNAME=
AUTH_EMAIL_PASSWORD=
AUTH_EMAIL_FROM=
AUTH_EMAIL_BASE_URL=

## Google OAuth
- Get from: https://console.cloud.google.com/apis/credentials
AUTH_OAUTH_GOOGLE_CLIENT_ID=
AUTH_OAUTH_GOOGLE_CLIENT_SECRET=
AUTH_OAUTH_GOOGLE_REDIRECT_URL=

## GitHub OAuth
- Get from: https://github.com/settings/developers
AUTH_OAUTH_GITHUB_CLIENT_ID=
AUTH_OAUTH_GITHUB_CLIENT_SECRET=
AUTH_OAUTH_GITHUB_REDIRECT_URL=
