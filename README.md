
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
