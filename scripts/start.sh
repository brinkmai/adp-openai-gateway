#!/bin/bash
SERVICE_NAME="adp-openai-gateway"
if [ "$EUID" -eq 0 ]; then
    systemctl start "$SERVICE_NAME"
else
    systemctl --user start "$SERVICE_NAME"
fi
echo "$SERVICE_NAME 已启动"
