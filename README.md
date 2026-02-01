# ADP OpenAI Gateway

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org)
[![Release](https://img.shields.io/github/v/release/brinkmai/adp-openai-gateway)](https://github.com/brinkmai/adp-openai-gateway/releases)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

将腾讯云 ADP（智能对话平台）转换为 OpenAI 兼容 API 的网关服务，让任何支持 OpenAI API 的客户端都能直接对接腾讯云 AI 服务。

## 功能特性

- ✅ 完全兼容 OpenAI Chat Completions API
- ✅ 支持流式输出（SSE）
- ✅ 支持非流式输出
- ✅ 自动 Token 缓存与刷新
- ✅ WebSocket 连接复用
- ✅ systemd 服务管理

## 架构原理

```
┌─────────────────┐      HTTP/SSE      ┌─────────────────┐     WebSocket      ┌─────────────────┐
│  OpenAI 客户端   │ ───────────────── │   Gateway 网关   │ ◀───────────────▶ │  腾讯云 ADP 服务  │
│  (Cursor等)     │                    │   (本项目)       │  Socket.IO v4     │  (LKE 大模型)    │
└─────────────────┘                    └─────────────────┘                    └─────────────────┘
```

## 与第三方客户端集成

在支持自定义 OpenAI API 的客户端中设置：

- **API Base URL**: `http://127.0.0.1:3100/v1`
- **API Key**: 任意值（本网关不校验）

支持的客户端包括但不限于：Cursor、Continue、ChatGPT Next Web、LobeChat 等。

## 快速开始

### 前置条件

- Go 1.21+（仅源码编译需要）
- 腾讯云账号（获取 SecretId/SecretKey）
- ADP 智能体应用（获取 BotAppKey）

### 方式一：从源码编译

```bash
git clone https://github.com/brinkmai/adp-openai-gateway.git
cd adp-openai-gateway

# 编辑配置
cp .env.example .env
vim .env

# 安装并启动
./scripts/install.sh
./scripts/start.sh
```

### 方式二：下载预编译版本

#### 一键安装（推荐）

```bash
# 创建安装目录
mkdir -p ~/adp-openai-gateway && cd ~/adp-openai-gateway

# 第一步：下载文件（首次运行会创建 .env 并退出）
curl -sSL https://raw.githubusercontent.com/brinkmai/adp-openai-gateway/main/scripts/install-binary.sh | bash

# 第二步：编辑配置
vim .env

# 第三步：完成安装并启动
curl -sSL https://raw.githubusercontent.com/brinkmai/adp-openai-gateway/main/scripts/install-binary.sh | bash
./scripts/start.sh
```

#### 手动安装

```bash
# Linux x86_64
curl -LO https://github.com/brinkmai/adp-openai-gateway/releases/latest/download/adp-openai-gateway-linux-amd64
chmod +x adp-openai-gateway-linux-amd64

# 下载配置模板
curl -LO https://raw.githubusercontent.com/brinkmai/adp-openai-gateway/main/.env.example
cp .env.example .env
vim .env

# 直接运行（前台）
./adp-openai-gateway-linux-amd64

# 或后台运行
nohup ./adp-openai-gateway-linux-amd64 > gateway.log 2>&1 &
```

## 配置说明

| 变量 | 说明 | 必填 |
|-----|------|-----|
| `SECRET_ID` | 腾讯云 SecretId | ✅ |
| `SECRET_KEY` | 腾讯云 SecretKey | ✅ |
| `ADP_BOT_APP_KEY` | ADP 智能体应用 Key | ✅ |
| `PORT` | 服务端口 | 默认 3100 |
| `HOST` | 监听地址 | 默认 127.0.0.1 |

## API 使用

### 非流式请求

```bash
curl -X POST http://127.0.0.1:3100/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"你好"}]}'
```

### 流式请求

```bash
curl -X POST http://127.0.0.1:3100/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"你好"}],"stream":true}'
```

## 服务管理

```bash
# 使用脚本（两种安装方式通用）
./scripts/start.sh    # 启动
./scripts/stop.sh     # 停止
./scripts/restart.sh  # 重启

# 或使用 systemctl
systemctl --user status adp-openai-gateway
systemctl --user restart adp-openai-gateway
journalctl --user -u adp-openai-gateway -f
```

## License

[MIT License](LICENSE)
