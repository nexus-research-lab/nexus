ARG GO_IMAGE=golang:1.26.2-bookworm
ARG NODE_IMAGE=node:22-bookworm-slim

FROM ${GO_IMAGE} AS builder

LABEL authors="leemysw"
LABEL description="Nexus"

ENV DEBIAN_FRONTEND=noninteractive \
    CGO_ENABLED=1

ARG APT_MIRROR=http://mirrors.tuna.tsinghua.edu.cn
ARG DEBIAN_SECURITY_MIRROR=http://mirrors.tuna.tsinghua.edu.cn/debian-security
ARG GOPROXY=https://goproxy.cn,direct
ARG PRIVATE_SDK_MODULE=github.com/nexus-research-lab/nexus-agent-sdk-go

RUN --mount=type=cache,target=/var/cache/apt \
    if [ -f /etc/apt/sources.list.d/debian.sources ]; then \
        sed -i -E "s#^URIs: https?://deb.debian.org/debian\$#URIs: ${APT_MIRROR}/debian#g" /etc/apt/sources.list.d/debian.sources && \
        sed -i -E "s#^URIs: https?://deb.debian.org/debian-security\$#URIs: ${DEBIAN_SECURITY_MIRROR}#g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    if [ -f /etc/apt/sources.list ]; then \
        sed -i -E "s#https?://deb.debian.org/debian-security#${DEBIAN_SECURITY_MIRROR}#g" /etc/apt/sources.list && \
        sed -i -E "s#https?://deb.debian.org/debian#${APT_MIRROR}/debian#g" /etc/apt/sources.list; \
    fi && \
    apt-get update -o Acquire::Retries=3 && \
    apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    git && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum /src/
COPY --from=nexus_agent_sdk . /src/.private-sdk

ENV GOPROXY=${GOPROXY}

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    test -f /src/.private-sdk/go.mod && \
    go mod edit -replace ${PRIVATE_SDK_MODULE}=/src/.private-sdk && \
    go mod download

COPY cmd /src/cmd
COPY internal /src/internal
COPY db /src/db

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -o /out/nexus-server ./cmd/nexus-server && \
    go build -o /out/nexus-migrate ./cmd/nexus-migrate && \
    go build -o /out/nexusctl ./cmd/nexusctl

FROM ${NODE_IMAGE} AS runtime

LABEL authors="leemysw"
LABEL description="Nexus"

ENV DEBIAN_FRONTEND=noninteractive

ARG APT_MIRROR=http://mirrors.tuna.tsinghua.edu.cn
ARG DEBIAN_SECURITY_MIRROR=http://mirrors.tuna.tsinghua.edu.cn/debian-security
ARG PNPM_REGISTRY=https://registry.npmmirror.com/
ARG PNPM_VERSION=9.15.2
ARG PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple
ARG BUN_CONFIG_REGISTRY=https://registry.npmmirror.com/

RUN --mount=type=cache,target=/var/cache/apt \
    if [ -f /etc/apt/sources.list.d/debian.sources ]; then \
        sed -i -E "s#^URIs: https?://deb.debian.org/debian\$#URIs: ${APT_MIRROR}/debian#g" /etc/apt/sources.list.d/debian.sources && \
        sed -i -E "s#^URIs: https?://deb.debian.org/debian-security\$#URIs: ${DEBIAN_SECURITY_MIRROR}#g" /etc/apt/sources.list.d/debian.sources; \
    fi && \
    if [ -f /etc/apt/sources.list ]; then \
        sed -i -E "s#https?://deb.debian.org/debian-security#${DEBIAN_SECURITY_MIRROR}#g" /etc/apt/sources.list && \
        sed -i -E "s#https?://deb.debian.org/debian#${APT_MIRROR}/debian#g" /etc/apt/sources.list; \
    fi && \
    apt-get update -o Acquire::Retries=3 && \
    apt-get install -y --no-install-recommends \
    bash \
    build-essential \
    ca-certificates \
    curl \
    git \
    jq \
    pkg-config \
    python3 \
    python3-dev \
    python3-pip \
    python3-venv \
    sudo && \
    ln -sf /usr/bin/python3 /usr/local/bin/python && \
    rm -rf /var/lib/apt/lists/*

ENV HOME=/home/agent \
    NEXUS_APP_ROOT=/opt/app \
    PNPM_HOME=/usr/local/share/pnpm \
    PATH=/usr/local/share/pnpm:/home/agent/.local/bin:/home/agent/.bun/bin:${PATH} \
    COREPACK_NPM_REGISTRY=${PNPM_REGISTRY} \
    PNPM_REGISTRY=${PNPM_REGISTRY} \
    BUN_CONFIG_REGISTRY=${BUN_CONFIG_REGISTRY} \
    PIP_INDEX_URL=${PIP_INDEX_URL} \
    PIP_BREAK_SYSTEM_PACKAGES=1 \
    UV_BREAK_SYSTEM_PACKAGES=true \
    UV_DEFAULT_INDEX=${PIP_INDEX_URL} \
    UV_INDEX_URL=${PIP_INDEX_URL}

RUN --mount=type=cache,target=/root/.local/share/pnpm/store,sharing=locked \
    corepack enable && \
    corepack prepare "pnpm@${PNPM_VERSION}" --activate && \
    pnpm config set registry "${PNPM_REGISTRY}" && \
    pnpm config set store-dir /root/.local/share/pnpm/store && \
    pnpm add -g @anthropic-ai/claude-code bun

RUN --mount=type=cache,target=/root/.cache/pip,sharing=locked \
    python3 -m pip install --no-cache-dir --break-system-packages \
    --index-url "${PIP_INDEX_URL}" \
    uv && \
    (python3 -m pip cache purge || true)

RUN printf 'registry=%s\n' "${PNPM_REGISTRY}" > /etc/npmrc && \
    mkdir -p /etc/uv && \
    printf '[[index]]\nurl = "%s"\ndefault = true\n\n[pip]\nindex-url = "%s"\n' "${PIP_INDEX_URL}" "${PIP_INDEX_URL}" > /etc/uv/uv.toml

RUN groupadd --system --gid 1001 agent && \
    useradd --system --uid 1001 --gid agent --shell /bin/bash --create-home agent && \
    mkdir -p /opt/app/deploy /home/agent/.nexus /home/agent/.claude /home/agent/.config/pip /home/agent/.config/uv /home/agent/.cache/pip

WORKDIR /opt/app

COPY --from=builder /out/nexus-server /usr/local/bin/nexus-server
COPY --from=builder /out/nexus-migrate /usr/local/bin/nexus-migrate
COPY --from=builder /out/nexusctl /usr/local/bin/nexusctl
COPY db /opt/app/db
COPY skills /opt/app/skills
COPY deploy/entrypoint.sh /opt/app/deploy/entrypoint.sh
COPY deploy/nexus-apt-install /usr/local/bin/nexus-apt-install

RUN chmod +x /opt/app/deploy/entrypoint.sh /usr/local/bin/nexus-apt-install && \
    chown root:root /usr/local/bin/nexus-apt-install && \
    printf '%s\n' \
    'Defaults:agent !requiretty' \
    'Defaults:agent env_keep += "NEXUS_APT_ALLOWLIST"' \
    'agent ALL=(root) NOPASSWD: /usr/local/bin/nexus-apt-install' \
    > /etc/sudoers.d/nexus-agent && \
    chmod 0440 /etc/sudoers.d/nexus-agent && \
    visudo -cf /etc/sudoers.d/nexus-agent && \
    chown -R agent:agent /opt/app /home/agent

USER agent

EXPOSE 8010

ENTRYPOINT ["/opt/app/deploy/entrypoint.sh"]
CMD ["nexus-server"]
