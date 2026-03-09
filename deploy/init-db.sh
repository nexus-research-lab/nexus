#!/bin/bash
# 数据库初始化脚本

set -e

echo "Initializing database..."

# 设置变量
DB_DIR="cache/data"
DB_FILE="${DB_DIR}/nexus-core.db"
ALEMBIC_INI="alembic.ini"

# 创建数据库目录
mkdir -p "${DB_DIR}"

# 确保 alembic.ini 中的 URL 是正确的
if [ -f "${ALEMBIC_INI}" ]; then
    sed -i "s|sqlalchemy.url = .*|sqlalchemy.url = sqlite+aiosqlite:///${DB_DIR}/nexus-core.db|g" "${ALEMBIC_INI}"
fi

# 检查数据库文件是否存在
if [ -f "${DB_FILE}" ]; then
    echo "Database already exists at ${DB_FILE}"
    echo "Checking database status..."

    # 检查数据库是否需要迁移
    if alembic check 2>/dev/null; then
        echo "Database is up to date."
    else
        echo "Database needs migration. Running migrations..."
        # 如果数据库需要更新，运行迁移
        alembic upgrade head
        echo "Migration completed."
    fi
else
    echo "Database not found. Creating new database..."

    # 检查 alembic.ini 是否存在
    if [ ! -f "${ALEMBIC_INI}" ]; then
        echo "Alembic configuration not found. Please ensure alembic.ini is in the project root."
        exit 1
    fi

    # 检查 alembic 目录是否存在
    if [ ! -d "/opt/app/alembic" ]; then
        echo "Initializing Alembic..."
        alembic init -t async alembic
    fi

    # 确保 alembic.ini 中的 URL 是正确的（再次设置）
    sed -i "s|sqlalchemy.url = .*|sqlalchemy.url = sqlite+aiosqlite:///${DB_DIR}/nexus-core.db|g" "${ALEMBIC_INI}"

    # 检查是否已经有迁移文件
    if [ ! -d "alembic/versions" ] || [ -z "$(ls -A alembic/versions)" ]; then
        echo "Creating initial migration..."
        alembic revision --autogenerate -m "Initial database schema"
    fi

    # 应用迁移
    echo "Applying migration..."
    alembic upgrade head
fi

echo "Database initialization completed successfully!"