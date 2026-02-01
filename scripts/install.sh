#!/bin/bash
# ADP-OpenAI Gateway (Go) 安装脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

SERVICE_NAME="adp-openai-gateway"
BINARY_NAME="adp-openai-gateway"

echo "=== ADP-OpenAI Gateway (Go) 安装 ==="

# 检查Go
if ! command -v go &> /dev/null; then
    echo "错误: 未安装Go，请先安装Go 1.21+"
    exit 1
fi

GO_VERSION=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1)
GO_MAJOR=$(echo "$GO_VERSION" | cut -d'.' -f1)
GO_MINOR=$(echo "$GO_VERSION" | cut -d'.' -f2)
if [ "$GO_MAJOR" -lt 1 ] || ([ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 21 ]); then
    echo "错误: Go版本过低，需要1.21+，当前: go$GO_VERSION"
    exit 1
fi

# 下载依赖并编译
echo "下载依赖..."
go mod tidy

echo "编译..."
go build -o "$BINARY_NAME" ./cmd/server

# 配置文件
if [ ! -f ".env" ]; then
    cp .env.example .env
    echo ""
    echo "=========================================="
    echo "请编辑 .env 填入腾讯云ADP配置:"
    echo "  SECRET_ID         - 腾讯云SecretId"
    echo "  SECRET_KEY        - 腾讯云SecretKey"
    echo "  ADP_BOT_APP_KEY   - ADP智能体应用Key"
    echo "=========================================="
    echo ""
    echo "配置完成后，请重新运行此脚本完成安装。"
    exit 0
fi

# 判断是否为 root 用户
if [ "$EUID" -eq 0 ]; then
    SYSTEMD_DIR="/etc/systemd/system"
    SYSTEMCTL_CMD="systemctl"
    JOURNALCTL_CMD="journalctl"
else
    SYSTEMD_DIR="$HOME/.config/systemd/user"
    SYSTEMCTL_CMD="systemctl --user"
    JOURNALCTL_CMD="journalctl --user"
fi

mkdir -p "$SYSTEMD_DIR"

ENV_FILE="$PROJECT_DIR/.env"

cat > "$SYSTEMD_DIR/${SERVICE_NAME}.service" << EOF
[Unit]
Description=ADP-OpenAI Gateway (Go)
After=network.target

[Service]
Type=simple
WorkingDirectory=$PROJECT_DIR
EnvironmentFile=$ENV_FILE
ExecStart=$PROJECT_DIR/$BINARY_NAME
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
EOF

# 重载systemd
$SYSTEMCTL_CMD daemon-reload
$SYSTEMCTL_CMD enable "$SERVICE_NAME"

echo "=== 安装完成 ==="
echo ""
echo "服务管理命令:"
echo "  启动: $SYSTEMCTL_CMD start $SERVICE_NAME"
echo "  停止: $SYSTEMCTL_CMD stop $SERVICE_NAME"
echo "  重启: $SYSTEMCTL_CMD restart $SERVICE_NAME"
echo "  状态: $SYSTEMCTL_CMD status $SERVICE_NAME"
echo "  日志: $JOURNALCTL_CMD -u $SERVICE_NAME -f"
echo ""
echo "或使用脚本:"
echo "  scripts/start.sh  scripts/stop.sh  scripts/restart.sh"
