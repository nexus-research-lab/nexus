ARG PYTHON_IMAGE=python:3.13

FROM ${PYTHON_IMAGE} AS runtime

LABEL authors="leemysw"
LABEL description="Nexus"

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PYTHONPATH=/opt/app

ARG APT_MIRROR=https://mirrors.tuna.tsinghua.edu.cn
ARG DEBIAN_SECURITY_MIRROR=https://mirrors.tuna.tsinghua.edu.cn/debian-security
ARG PIP_INDEX_URL=https://pypi.tuna.tsinghua.edu.cn/simple
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
    git \
    nodejs \
    npm && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /opt/app

COPY agent/requirements.txt /tmp/requirements.txt

RUN --mount=type=cache,target=/root/.cache/pip \
    pip config set global.index-url "${PIP_INDEX_URL}" && \
    pip install --upgrade pip && \
    pip install -r /tmp/requirements.txt

RUN npm config set registry "${NPM_REGISTRY}" && \
    npm install -g @anthropic-ai/claude-code --registry "${NPM_REGISTRY}" && \
    npm cache clean --force

RUN groupadd --system --gid 1001 agent && \
    useradd --system --uid 1001 --gid agent --shell /bin/bash --create-home agent && \
    mkdir -p /opt/app/deploy /home/agent/.nexus /home/agent/.claude

COPY main.py pyproject.toml alembic.ini /opt/app/
COPY alembic /opt/app/alembic
COPY agent /opt/app/agent
COPY skills /opt/app/skills
COPY deploy/init-db.sh /opt/app/deploy/init-db.sh

RUN chmod +x /opt/app/deploy/init-db.sh && \
    chown -R agent:agent /opt/app /home/agent

USER agent
ENV HOME=/home/agent

EXPOSE 8010

ENTRYPOINT ["/opt/app/deploy/init-db.sh"]
CMD ["gunicorn", "-c", "agent/config/config_gunicorn.py", "main:app"]
