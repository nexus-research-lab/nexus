# syntax=docker/dockerfile:1.7
ARG GO_IMAGE=golang:1.26.2-bookworm
ARG NODE_IMAGE=node:22-bookworm-slim

FROM ${GO_IMAGE} AS builder

LABEL authors="leemysw"
LABEL description="Nexus"

ENV DEBIAN_FRONTEND=noninteractive \
    CGO_ENABLED=0

ARG APT_MIRROR=http://mirrors.tuna.tsinghua.edu.cn
ARG DEBIAN_SECURITY_MIRROR=http://mirrors.tuna.tsinghua.edu.cn/debian-security
ARG GOPROXY=https://goproxy.cn,direct

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
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
    ca-certificates \
    git && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum /src/

ENV GOPROXY=${GOPROXY}

# 模块下载保留在可复用的依赖层；仅依赖 go.mod/go.sum 变化时才重新下载。
RUN go mod download

COPY cmd /src/cmd
COPY internal /src/internal
COPY db /src/db

RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -o /out/nexus-server ./cmd/nexus-server && \
    go build -o /out/nexusctl ./cmd/nexusctl && \
    go build -o /out/nexus-runtime-launcher ./cmd/nexus-runtime-launcher

FROM ${NODE_IMAGE} AS runtime-base

LABEL authors="leemysw"
LABEL description="Nexus"

ENV DEBIAN_FRONTEND=noninteractive

ARG APT_MIRROR=http://mirrors.tuna.tsinghua.edu.cn
ARG DEBIAN_SECURITY_MIRROR=http://mirrors.tuna.tsinghua.edu.cn/debian-security
ARG PNPM_REGISTRY=https://registry.npmjs.org/
ARG PNPM_VERSION=9.15.2
ARG PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple
ARG BUN_CONFIG_REGISTRY=https://registry.npmjs.org/

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
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
    acl \
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
    NEXUSCTL_COMMAND_PATH=/usr/local/bin/nexusctl \
    NEXUS_NXS_COMMAND_PATH=/opt/app/bin/nxs \
    PNPM_HOME=/usr/local/share/pnpm \
    PATH=/usr/local/share/pnpm:/home/agent/.local/bin:/home/agent/.bun/bin:${PATH} \
    PNPM_REGISTRY=${PNPM_REGISTRY} \
    BUN_CONFIG_REGISTRY=${BUN_CONFIG_REGISTRY} \
    PIP_INDEX_URL=${PIP_INDEX_URL} \
    PIP_BREAK_SYSTEM_PACKAGES=1 \
    UV_BREAK_SYSTEM_PACKAGES=true \
    UV_DEFAULT_INDEX=${PIP_INDEX_URL} \
    UV_INDEX_URL=${PIP_INDEX_URL}

RUN --mount=type=cache,target=/root/.local/share/pnpm/store,sharing=locked \
    --mount=type=cache,target=/root/.npm,sharing=locked \
    npm install --global --no-audit --no-fund "pnpm@${PNPM_VERSION}" --registry "${PNPM_REGISTRY}" && \
    pnpm config set registry "${PNPM_REGISTRY}" && \
    pnpm config set store-dir /root/.local/share/pnpm/store && \
    pnpm add -g @anthropic-ai/claude-code bun

RUN claude_path="$(command -v claude)" && \
    test -x "${claude_path}" && \
    if [ "${claude_path}" != "/usr/local/bin/claude" ]; then \
        printf '#!/bin/sh\nexec "%s" "$@"\n' "${claude_path}" > /usr/local/bin/claude && \
        chmod 0755 /usr/local/bin/claude; \
    fi && \
    /usr/local/bin/claude --version

RUN --mount=type=cache,target=/root/.cache/pip,sharing=locked \
    python3 -m pip install --break-system-packages \
    --retries 5 \
    --timeout 120 \
    --index-url "${PIP_INDEX_URL}" \
    uv

RUN printf 'registry=%s\n' "${PNPM_REGISTRY}" > /etc/npmrc && \
    mkdir -p /etc/uv && \
    printf '[[index]]\nurl = "%s"\ndefault = true\n\n[pip]\nindex-url = "%s"\n' "${PIP_INDEX_URL}" "${PIP_INDEX_URL}" > /etc/uv/uv.toml

RUN groupadd --system --gid 1001 agent && \
    useradd --system --uid 1001 --gid agent --shell /bin/bash --create-home agent && \
    mkdir -p /opt/app/deploy /home/agent/.nexus /home/agent/.nexus/users/__system__/runtime/home /home/agent/.nexus/users/__system__/runtime/logs /home/agent/.nexus/users/__system__/runtime/.claude /home/agent/.claude /home/agent/.config/pip /home/agent/.config/uv /home/agent/.cache/pip

WORKDIR /opt/app

# nxs runtime 只依赖下载脚本、目标平台和 release。放在业务源码之前，
# 避免 Go、Skill 或入口脚本变化触发重复下载。
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG NEXUS_DOCKER_BUNDLE_NXS_RUNTIME=1
ARG NEXUS_NXS_RUNTIME_RELEASE=nxs-stable
COPY --chown=root:root --chmod=0444 scripts/desktop/fetch-nxs-runtime.js /opt/app/scripts/desktop/fetch-nxs-runtime.js
RUN if [ "${NEXUS_DOCKER_BUNDLE_NXS_RUNTIME}" = "1" ]; then \
        mkdir -p /opt/app/bin && \
        node /opt/app/scripts/desktop/fetch-nxs-runtime.js \
        --goos "${TARGETOS:-linux}" \
        --goarch "${TARGETARCH:-amd64}" \
        --release "${NEXUS_NXS_RUNTIME_RELEASE}" \
        --output /opt/app/bin/nxs \
        --ripgrep-output /opt/app/bin/rg; \
    fi && \
    find /opt/app/scripts -type d -exec chmod 0555 {} \; && \
    if [ -f /opt/app/bin/nxs ]; then \
        chown root:root /opt/app/bin/nxs /opt/app/bin/rg && \
        chmod 0555 /opt/app/bin /opt/app/bin/nxs /opt/app/bin/rg; \
    fi

# OS 账号、launcher 配置与 sudo policy 不依赖产品源码，作为稳定基础层缓存。
RUN test -x /usr/local/bin/claude && \
    mkdir -p /etc/nexus && \
    printf '%s\n' \
    '{' \
    '  "state_root": "/home/agent/.nexus",' \
    '  "host_uid": 1001,' \
    '  "host_gid": 1001,' \
    '  "uid_min": 20000,' \
	    '  "uid_max": 59999,' \
	    '  "ticket_ttl_seconds": 86400,' \
	    '  "landlock_required": true,' \
	    '  "cgroup_root": "",' \
	    '  "cgroup_required": false,' \
    '  "runtime_executables": {' \
    '    "nxs": "/opt/app/bin/nxs",' \
    '    "claude": "/usr/local/bin/claude",' \
    '    "script": "/bin/sh"' \
    '  },' \
    '  "read_only_roots": [' \
    '    "/opt/app",' \
    '    "/home/agent/.nexus/app/platform-skills"' \
    '  ]' \
    '}' \
    > /etc/nexus/runtime-isolation.json && \
    chown root:root /etc/nexus/runtime-isolation.json && \
    chmod 0644 /etc/nexus/runtime-isolation.json && \
    printf '%s\n' \
    'Defaults:agent !requiretty' \
    'Defaults:agent env_keep += "NEXUS_APT_ALLOWLIST"' \
    'agent ALL=(root) NOPASSWD: /usr/local/bin/nexus-apt-install' \
    > /etc/sudoers.d/nexus-agent && \
    chmod 0440 /etc/sudoers.d/nexus-agent && \
    visudo -cf /etc/sudoers.d/nexus-agent && \
    chown -R agent:agent /home/agent && \
    chmod 0751 /home/agent

# 数据库与 Skill 树各自归一权限；后续业务二进制变化不会让 nxs 下载层失效。
COPY --chown=root:root db /opt/app/db
RUN find /opt/app/db -type d -exec chmod 0555 {} \; && \
    find /opt/app/db -type f -exec chmod a-w {} \;

COPY --chown=root:root skills /opt/app/skills
RUN find /opt/app/skills -type d -exec chmod 0555 {} \; && \
    find /opt/app/skills -type f -exec chmod a-w {} \;

COPY --from=builder --chown=root:root --chmod=0755 /out/nexus-server /usr/local/bin/nexus-server
COPY --from=builder --chown=root:agent --chmod=0750 /out/nexusctl /usr/local/bin/nexusctl
COPY --from=builder --chown=root:agent --chmod=4750 /out/nexus-runtime-launcher /usr/local/libexec/nexus-runtime-launcher
COPY --chown=root:root --chmod=0555 deploy/entrypoint.sh /opt/app/deploy/entrypoint.sh
COPY --chown=root:root --chmod=0755 deploy/nexus-apt-install /usr/local/bin/nexus-apt-install

USER agent

EXPOSE 8010

ENTRYPOINT ["/opt/app/deploy/entrypoint.sh"]
CMD ["nexus-server"]

FROM ${NODE_IMAGE} AS web-builder

ARG PNPM_REGISTRY=https://registry.npmjs.org/
ARG PNPM_VERSION=9.15.2
ARG VITE_WS_URL=
ARG VITE_API_URL=

ENV VITE_WS_URL=${VITE_WS_URL} \
    VITE_API_URL=${VITE_API_URL} \
    PNPM_HOME=/pnpm \
    PATH=/pnpm:${PATH}

WORKDIR /src/web

COPY web/package.json web/pnpm-lock.yaml ./
COPY web/.npmrc ./
RUN --mount=type=cache,target=/pnpm/store,sharing=locked \
    npm install --global --no-audit --no-fund "pnpm@${PNPM_VERSION}" --registry "${PNPM_REGISTRY}" && \
    pnpm config set registry "${PNPM_REGISTRY}" && \
    pnpm config set store-dir /pnpm/store && \
    pnpm install --frozen-lockfile

COPY web/ ./
COPY docs/guides/room-skill-authoring.en.md /src/docs/guides/room-skill-authoring.en.md
COPY docs/guides/room-skill-authoring.md /src/docs/guides/room-skill-authoring.md
RUN pnpm run build

FROM runtime-base AS runtime

COPY --from=web-builder --chown=agent:agent /src/web/dist /opt/app/web/dist
