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
  "interval": "3m",
  "dns": {
    "provider": "smartdns",
    "config": {
      "conf_file": "/var/etc/smartdns/smartdns.conf",
      "output_file": "/tmp/lucky-domains.conf",
      "reload_cmd": "kill $(pidof smartdns) 2>/dev/null; rm -f /etc/smartdns/smartdns.cache; /usr/sbin/smartdns -f -c /var/etc/smartdns/smartdns.conf &"
    }
  },
  "rule_sources": [
    {
      "type": "lucky_api",
      "url": "https://192.168.100.1:2053/webservice/rules",
      "target": "https://192.168.100.1:2053",
      "proto": "prefer-https",
      "filter": {
        "rule_name": "Lucky_https",
        "listen_port": 2053,
        "domain_contains": "46."
      },
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

### 配置项说明

#### 基础配置

- **`lan_ip`** - 内网 IP 地址（如 `192.168.100.1`），用于 SmartDNS 域名解析
- **`log_file`** - 日志文件路径，归档目录为 `log_file` 同目录下的 `logs/`（如 `/tmp/hproxy/hproxy.log` → 归档到 `/tmp/hproxy/logs/`）
- **`cert`** - HTTPS 证书文件路径
- **`key`** - HTTPS 私钥文件路径
- **`admin_port`** - 管理接口监听端口（如 `18080`）
- **`interval`** - 定时更新间隔（如 `3m` 表示 3 分钟，默认 3 分钟）

#### 代理服务器配置

- **`proxy_servers`** - 代理监听地址数组，支持以下格式：
  - `http://:80` - 纯 HTTP 代理
  - `https://:443` - 纯 HTTPS 代理
  - `:4555` - 自动模式（同一个端口同时处理 HTTP 和 HTTPS，需要证书）

#### 调试配置

- **`debug`** - 是否打印调试日志（可选，默认 `false`）
  - `false` - 只打印重要日志（代理请求、错误、DNS 更新结果）
  - `true` - 额外打印调试日志（规则加载详情、DNS 变更检测、定时任务等）

```json
{
  "debug": false
}
```

#### DNS 配置

- **`dns.provider`** - DNS 提供商名称（目前支持 `smartdns`）
- **`dns.config.conf_file`** - SmartDNS 主配置文件路径
- **`dns.config.output_file`** - SmartDNS 域名配置文件路径（hproxy 写入）
- **`dns.config.reload_cmd`** - SmartDNS 重载命令（可选，为空则跳过重载；默认 `kill $(pidof smartdns) 2>/dev/null; rm -f /etc/smartdns/smartdns.cache; /usr/sbin/smartdns -f -c /var/etc/smartdns/smartdns.conf &`）

#### 规则来源配置

- **`rule_sources`** - 规则来源数组，支持两种类型：

##### 类型 1：`lucky_api`（Lucky API）

从 Lucky 软路由的 API 获取规则：

```json
{
  "type": "lucky_api",
  "url": "https://192.168.100.1:2053/webservice/rules",
  "target": "https://192.168.100.1:2053",
  "proto": "prefer-https",
  "filter": {
    "rule_name": "Lucky_https",
    "listen_port": 2053,
    "domain_contains": "46."
  },
  "enabled": true
}
```

- `url` - Lucky API 地址
- `target` - 规则目标地址（回源地址）
- `proto` - 协议类型：`strict` / `prefer-https` / `prefer-http` / `force-https` / `force-http` / `direct`
- `filter` - 过滤条件（可选）
  - `rule_name` - 规则名称过滤
  - `listen_port` - 监听端口过滤
  - `domain_contains` - 域名包含字符串过滤

##### 类型 2：`local_file`（本地文件）

从本地 JSON 文件读取规则：

```json
{
  "type": "local_file",
  "path": "rules.json",
  "format": "json",
  "enabled": true
}
```

- `path` - 本地规则文件路径
- `format` - 文件格式（目前只支持 `json`）

#### 规则文件格式

`rules.json` 格式：

```json
[
  {
    "host": "example.com",
    "target": ["https://192.168.100.1:2053"],
    "proto": "prefer-https"
  },
  {
    "host": "*.example.com",
    "target": ["https://192.168.100.1:2053"],
    "proto": "prefer-https"
  }
]
```

- `host` - 域名（支持通配符 `*.example.com`）
- `target` - 目标地址数组
- `proto` - 协议类型

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
