#!/bin/bash
SERVICE_NAME="adp-openai-gateway"
if [ "$EUID" -eq 0 ]; then
    systemctl stop "$SERVICE_NAME"
else
    systemctl --user stop "$SERVICE_NAME"
fi
echo "$SERVICE_NAME 已停止"
