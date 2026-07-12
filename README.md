# hproxy

hproxy 是一个轻量级的 HTTP/HTTPS 反向代理，支持多规则来源、动态 DNS 更新、日志归档和管理接口。

## 功能

- **多规则来源**：支持 Lucky API 和本地文件两种规则来源
- **动态 DNS 更新**：自动更新 SmartDNS 配置，支持变更检测
- **日志按天归档**：日志按天自动归档，支持快速归档（`/logs/start`）
- **管理接口**：提供 RESTful 管理接口，查看规则、重载配置、查看日志
- **定时器更新**：每 3 分钟自动更新规则和 DNS（可配置）

## 安装

### 从 Release 下载

访问 [Releases](https://github.com/scyslz/hproxy/releases) 页面，下载适合你平台的二进制文件：

- `hproxy-linux-amd64` - Linux x86_64
- `hproxy-linux-arm64` - Linux ARM64（如 OpenWrt）

### 从源码编译

```bash
go build -o hproxy .
```

交叉编译（如 OpenWrt）：

```bash
GOOS=linux GOARCH=arm64 go build -o hproxy-arm64
```

## 配置

配置文件 `config.json`：

```json
{
  "lan_ip": "192.168.100.1",
  "log_file": "/tmp/hproxy/hproxy.log",
  "cert": "lucky-multi.crt",
  "key": "lucky-multi.key",
  "admin_port": "18080",
  "proxy_servers": [
    "http://:80",
    "https://:443"
  ],
  "dns": {
    "provider": "smartdns",
    "config": {
      "conf_file": "/var/etc/smartdns/smartdns.conf",
      "output_file": "/tmp/lucky-domains.conf"
    }
  },
  "rule_sources": [
    {
      "type": "lucky_api",
      "url": "https://192.168.100.1:2053/webservice/rules",
      "target": "https://192.168.100.1:2053",
      "proto": "prefer-https",
      "enabled": true
    },
    {
      "type": "local_file",
      "path": "rules.json",
      "format": "json",
      "enabled": true
    }
  ]
}
```

### 配置说明

- `lan_ip` - 内网 IP（如 192.168.100.1）
- `log_file` - 日志文件路径（归档目录为 `log_file` 同目录下的 `logs/`）
- `cert` / `key` - HTTPS 证书和密钥
- `admin_port` - 管理接口端口
- `proxy_servers` - 代理监听地址（支持 `http://`、 `https://` 或 `:port`）
- `dns` - DNS 提供商配置（目前支持 smartdns）
- `rule_sources` - 规则来源配置

## 运行

```bash
./hproxy config.json
```

或者使用启动脚本：

```bash
./start-hproxy-v2.sh
```

## 管理接口

管理接口默认监听 `:18080`，提供以下接口：

- `GET /config` - 查看配置
- `GET /rules` - 查看规则（含来源信息）
- `POST /reload` - 重载配置和规则
- `GET /logs` - 查看最新日志内容
- `POST /logs/start` - 快速归档（当前日志归档为 `YYYY-MM-DD_001.log`，创建新文件）

### 示例

```bash
# 查看规则
curl http://127.0.0.1:18080/rules

# 重载配置
curl -X POST http://127.0.0.1:18080/reload

# 快速归档日志
curl -X POST http://127.0.0.1:18080/logs/start
```

## 日志归档

日志按天归档到 `log_file` 同目录下的 `logs/` 目录：

- 当前日志：`logs/2026-07-12.log`
- 归档日志：`logs/2026-07-12_001.log`、`logs/2026-07-12_002.log`...

快速归档（`/logs/start`）：
1. 当前日志文件立即归档（加序号）
2. 创建新的日志文件（原文件名）
3. 后续日志写到新文件

## OpenWrt 部署

1. 交叉编译：`GOOS=linux GOARCH=arm64 go build -o hproxy-arm64`
2. 上传到 OpenWrt：`scp hproxy-arm64 root@192.168.100.1:/root/hp/v2/`
3. 启动：`./start-hproxy-v2.sh`

## 技术栈

- Go 1.21+
- SmartDNS（动态 DNS 更新）
- OpenWrt（测试环境）

## License

MIT
