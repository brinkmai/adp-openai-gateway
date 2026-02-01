#!/bin/bash
# ADP-OpenAI Gateway 预编译版本一键安装脚本

set -e

REPO="brinkmai/adp-openai-gateway"
SERVICE_NAME="adp-openai-gateway"

# 检测系统和架构
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    arm64)   ARCH="arm64" ;;
    *)       echo "不支持的架构: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux)  SUFFIX="" ;;
    darwin) SUFFIX="" ;;
    *)      echo "不支持的系统: $OS"; exit 1 ;;
esac

BINARY_NAME="${SERVICE_NAME}-${OS}-${ARCH}${SUFFIX}"
INSTALL_DIR="${INSTALL_DIR:-$(pwd)}"

echo "=== ADP-OpenAI Gateway 安装 ==="
echo "系统: $OS, 架构: $ARCH"
echo "安装目录: $INSTALL_DIR"
echo ""

cd "$INSTALL_DIR"

# 下载二进制文件
echo "下载 $BINARY_NAME ..."
curl -LO "https://github.com/${REPO}/releases/latest/download/${BINARY_NAME}"
chmod +x "$BINARY_NAME"
ln -sf "$BINARY_NAME" "$SERVICE_NAME"

# 下载配置模板
if [ ! -f ".env.example" ]; then
    echo "下载配置模板..."
    curl -LO "https://raw.githubusercontent.com/${REPO}/main/.env.example"
fi

# 下载管理脚本到 scripts/ 目录
echo "下载管理脚本..."
mkdir -p scripts
for script in start.sh stop.sh restart.sh; do
    curl -sL "https://raw.githubusercontent.com/${REPO}/main/scripts/${script}" -o "scripts/${script}"
    chmod +x "scripts/${script}"
done

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

# 配置 systemd
echo "配置 systemd 服务..."

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

cat > "$SYSTEMD_DIR/${SERVICE_NAME}.service" << EOF
[Unit]
Description=ADP-OpenAI Gateway
After=network.target

[Service]
Type=simple
WorkingDirectory=$INSTALL_DIR
EnvironmentFile=$INSTALL_DIR/.env
ExecStart=$INSTALL_DIR/$SERVICE_NAME
Restart=always
RestartSec=5

[Install]
WantedBy=default.target
EOF

$SYSTEMCTL_CMD daemon-reload
$SYSTEMCTL_CMD enable "$SERVICE_NAME"

echo ""
echo "=== 安装完成 ==="
echo ""
echo "服务管理命令:"
echo "  启动: $SYSTEMCTL_CMD start $SERVICE_NAME  或  ./scripts/start.sh"
echo "  停止: $SYSTEMCTL_CMD stop $SERVICE_NAME   或  ./scripts/stop.sh"
echo "  重启: $SYSTEMCTL_CMD restart $SERVICE_NAME 或  ./scripts/restart.sh"
echo "  状态: $SYSTEMCTL_CMD status $SERVICE_NAME"
echo "  日志: $JOURNALCTL_CMD -u $SERVICE_NAME -f"
