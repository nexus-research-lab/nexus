ARG GO_IMAGE=golang:1.24-bookworm
ARG NODE_IMAGE=node:22-bookworm-slim

FROM ${GO_IMAGE} AS builder

LABEL authors="leemysw"
LABEL description="Nexus"

ENV DEBIAN_FRONTEND=noninteractive \
    CGO_ENABLED=1 \
    GOFLAGS=-mod=vendor

ARG APT_MIRROR=https://mirrors.tuna.tsinghua.edu.cn
ARG DEBIAN_SECURITY_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian-security
ARG NPM_REGISTRY=https://registry.npmmirror.com/

RUN --mount=type=cache,target=/var/cache/apt \
    if [ -f /etc/apt/sources.list.d/debian.sources ]; then \
        sed -i -E "s#https?://deb.debian.org/debian#${APT_MIRROR}/debian#g" /etc/apt/sources.list.d/debian.sources && \
        sed -i -E "s#https?://(deb|security).debian.org/debian-security#${DEBIAN_SECURITY_MIRROR}#g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    if [ -f /etc/apt/sources.list ]; then \
        sed -i -E "s#https?://deb.debian.org/debian#${APT_MIRROR}/debian#g" /etc/apt/sources.list && \
        sed -i -E "s#https?://(deb|security).debian.org/debian-security#${DEBIAN_SECURITY_MIRROR}#g" /etc/apt/sources.list; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    git && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum /src/
COPY vendor /src/vendor
COPY cmd /src/cmd
COPY internal /src/internal
COPY db /src/db

RUN go mod edit -dropreplace github.com/nexus-research-lab/nexus-agent-sdk-go || true

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -o /out/nexus-server ./cmd/nexus-server && \
    go build -o /out/nexus-migrate ./cmd/nexus-migrate

FROM ${NODE_IMAGE} AS runtime

LABEL authors="leemysw"
LABEL description="Nexus"

ENV DEBIAN_FRONTEND=noninteractive

ARG APT_MIRROR=https://mirrors.tuna.tsinghua.edu.cn
ARG DEBIAN_SECURITY_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian-security
ARG NPM_REGISTRY=https://registry.npmmirror.com/

RUN --mount=type=cache,target=/var/cache/apt \
    if [ -f /etc/apt/sources.list.d/debian.sources ]; then \
        sed -i -E "s#https?://deb.debian.org/debian#${APT_MIRROR}/debian#g" /etc/apt/sources.list.d/debian.sources && \
        sed -i -E "s#https?://(deb|security).debian.org/debian-security#${DEBIAN_SECURITY_MIRROR}#g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    if [ -f /etc/apt/sources.list ]; then \
        sed -i -E "s#https?://deb.debian.org/debian#${APT_MIRROR}/debian#g" /etc/apt/sources.list && \
        sed -i -E "s#https?://(deb|security).debian.org/debian-security#${DEBIAN_SECURITY_MIRROR}#g" /etc/apt/sources.list; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git && \
    rm -rf /var/lib/apt/lists/*

RUN npm config set registry "${NPM_REGISTRY}" && \
    npm install -g @anthropic-ai/claude-code --registry "${NPM_REGISTRY}" && \
    npm cache clean --force

RUN groupadd --system --gid 1001 agent && \
    useradd --system --uid 1001 --gid agent --shell /bin/bash --create-home agent && \
    mkdir -p /opt/app/deploy /home/agent/.nexus /home/agent/.claude

WORKDIR /opt/app

COPY --from=builder /out/nexus-server /usr/local/bin/nexus-server
COPY --from=builder /out/nexus-migrate /usr/local/bin/nexus-migrate
COPY db /opt/app/db
COPY skills /opt/app/skills
COPY deploy/init-db.sh /opt/app/deploy/init-db.sh

RUN chmod +x /opt/app/deploy/init-db.sh && \
    chown -R agent:agent /opt/app /home/agent

USER agent
ENV HOME=/home/agent

EXPOSE 8010

ENTRYPOINT ["/opt/app/deploy/init-db.sh"]
CMD ["nexus-server"]
