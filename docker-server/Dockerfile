FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive

# ── Install dependencies ──────────────────────────────────────────────
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    git \
    make \
    sudo \
    socat \
    openssh-server \
    gettext-base \
    apt-transport-https \
    ca-certificates \
    gnupg \
    lsb-release \
    conntrack \
    iproute2 \
    iptables \
    && rm -rf /var/lib/apt/lists/*

# ── Install Docker ────────────────────────────────────────────────────
RUN curl -fsSL https://get.docker.com | sh

# ── Install kubectl ───────────────────────────────────────────────────
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" \
    && install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl \
    && rm kubectl

# ── Install Minikube ──────────────────────────────────────────────────
RUN curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 \
    && install minikube-linux-amd64 /usr/local/bin/minikube \
    && rm minikube-linux-amd64

# ── Install Go ────────────────────────────────────────────────────────
RUN wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz \
    && rm go1.22.0.linux-amd64.tar.gz

ENV PATH=$PATH:/usr/local/go/bin
ENV GOPATH=/root/go
ENV PATH=$PATH:$GOPATH/bin

# ── Install protoc + plugins ──────────────────────────────────────────
RUN apt-get update && apt-get install -y protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest \
    && go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

# ── Install grpcurl ───────────────────────────────────────────────────
RUN curl -sSL https://github.com/fullstorydev/grpcurl/releases/download/v1.8.9/grpcurl_1.8.9_linux_x86_64.tar.gz \
    | tar -xz -C /usr/local/bin

# ── Setup SSH ─────────────────────────────────────────────────────────
RUN mkdir /var/run/sshd
RUN echo 'root:root' | chpasswd
RUN sed -i 's/#PermitRootLogin prohibit-password/PermitRootLogin yes/' /etc/ssh/sshd_config
RUN sed -i 's/#PasswordAuthentication yes/PasswordAuthentication yes/' /etc/ssh/sshd_config

# ── Create workspace ──────────────────────────────────────────────────
WORKDIR /workspace

# ── Copy entrypoint ───────────────────────────────────────────────────
COPY docker/server/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 22 8080 50051

ENTRYPOINT ["/entrypoint.sh"]