# 构建阶段
ARG FROM_IMAGE_NAME=python:3.11-slim

FROM ${FROM_IMAGE_NAME} AS builder

LABEL authors="leemysw"
LABEL description="Nexus Core Backend"

# 设置环境变量
ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

# 安装构建依赖
RUN --mount=type=cache,target=/var/cache/apt \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    curl vim &&  \
    rm -rf /var/lib/apt/lists/* && \
    apt-get clean


# 设置工作目录
WORKDIR /opt/app

# 安装claude code
RUN curl -fsSL https://claude.ai/install.sh | bash

# 复制并安装 Python 依赖
COPY ../agent/requirements.txt ./requirements.txt

# 设置 pip 镜像源
ARG PIP_MIRROR=https://pypi.tuna.tsinghua.edu.cn/simple
RUN pip config set global.index-url ${PIP_MIRROR}

# 安装依赖
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --upgrade pip && \
    pip install -r requirements.txt

# 构建阶段 - 构建包
FROM builder AS package-builder

# 复制源代码
COPY pyproject.toml MANIFEST.in ./
COPY agent ./agent

# 安装构建工具并构建 wheel
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install build wheel && \
    python -m build --wheel


# 运行阶段
FROM builder

# 创建非 root 用户
RUN groupadd --gid 1001 agent && \
    useradd --uid 1001 --gid agent --shell /bin/bash --create-home agent

# 设置工作目录
WORKDIR /opt/app

# 从构建阶段复制构建的 wheel
COPY --from=package-builder /opt/app/dist/*.whl /tmp/

# 安装应用
RUN pip install /tmp/*.whl  --no-deps --force-reinstall && \
    rm -rf /tmp/*

# 创建启动脚本（在切换用户前创建）
RUN echo '#!/bin/bash\n\
# 确保数据库已初始化\n\
/opt/app/init-db.sh\n\
\n\
# 启动应用\n\
exec nexus-core' > /opt/app/entrypoint.sh

# 复制初始化脚本和 Alembic 配置
COPY ./deploy/init-db.sh /opt/app/init-db.sh
COPY alembic.ini /opt/app/alembic.ini
COPY alembic /opt/app/alembic

# 给脚本执行权限
RUN chmod +x /opt/app/init-db.sh /opt/app/entrypoint.sh

# 切换到非 root 用户
RUN chown -R agent:agent /opt/app
USER agent

# 暴露端口
EXPOSE 8010

# 设置默认命令
CMD ["/opt/app/entrypoint.sh"]