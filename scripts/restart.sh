#!/bin/bash
SERVICE_NAME="adp-openai-gateway"
if [ "$EUID" -eq 0 ]; then
    systemctl restart "$SERVICE_NAME"
else
    systemctl --user restart "$SERVICE_NAME"
fi
echo "$SERVICE_NAME 已重启"
