# Gateway
[![Build Status](https://github.com/aide-family/goddess/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/aide-family/goddess/actions/workflows/go.yml)
[![codecov](https://codecov.io/gh/go-kratos/gateway/branch/main/graph/badge.svg)](https://codecov.io/gh/go-kratos/gateway)

HTTP -> Proxy -> Router -> Middleware -> Client -> Selector -> Node

## Protocol
* HTTP -> HTTP  
* HTTP -> gRPC  
* gRPC -> gRPC  

## Encoding
* Protobuf Schemas

## Endpoint
* prefix: /api/echo/*
* path: /api/echo/hello
* regex: /api/echo/[a-z]+
* restful: /api/echo/{name}

## Middleware
* cors
* auth
* color
* logging
* tracing
* metrics
* ratelimit
* datacenter

## 可用的调试接口

1. Go pprof 性能分析（内置）

```
/debug/pprof/              # pprof 主页
/debug/pprof/profile       # CPU 性能分析
/debug/pprof/heap          # 内存堆分析
/debug/pprof/goroutine     # Goroutine 分析
/debug/pprof/allocs        # 内存分配分析
/debug/pprof/block         # 阻塞分析
/debug/pprof/mutex         # 互斥锁分析
/debug/pprof/trace         # 执行追踪
```

2. Proxy 调试接口

```
GET /debug/proxy/router/inspect
```

- 功能：查看当前路由表结构
- 返回：JSON 格式的路由配置信息

3. Config 调试接口

```
GET /debug/config/inspect    # 查看配置加载器状态
GET /debug/config/load       # 手动触发配置重载
GET /debug/config/version    # 查看配置版本
```

- inspect：显示配置文件路径、SHA256 哈希、优先级配置哈希等
- load：返回当前加载的完整配置（JSON）
- version：返回配置版本号

4. Control Service 调试接口（如果启用）

```
GET /debug/ctrl/inspect      # 查看控制服务加载器状态
POST /debug/ctrl/load        # 手动触发从控制服务拉取配置
```

- inspect：显示控制服务地址、当前索引、目标路径等
- load：手动触发从控制服务拉取最新配置

## 控制服务 API

Gateway 支持通过控制服务（Control Service）进行集中式配置管理和动态更新。当启用控制服务时，Gateway 会定期从控制服务拉取配置，实现无需重启的动态配置更新。

### 启用控制服务

启动 Gateway 时通过以下参数启用控制服务：

```bash
./gateway \
  --ctrl.name=my-gateway \
  --ctrl.service=http://control-service-1:8000,http://control-service-2:8000 \
  --conf.priority=./canary
```

**参数说明：**
- `--ctrl.name`：Gateway 名称，用于标识当前 Gateway 实例（也可通过 `ADVERTISE_NAME` 环境变量设置）
- `--ctrl.service`：控制服务地址，支持多个地址用逗号分隔（自动负载均衡和故障转移）
- `--conf.priority`：优先级配置目录，用于灰度发布（可选）

**环境变量：**
- `ADVERTISE_NAME`：Gateway 名称
- `ADVERTISE_ADDR`：Gateway IP 地址（如果不设置，会自动检测 `eth0` 网卡的 IP）
- `ADVERTISE_DEVICE`：指定用于获取 IP 的网卡名称（默认：`eth0`）

### 控制服务需要实现的 API

控制服务需要提供以下两个 API 端点：

#### 1. 配置发布 API：`/v1/control/gateway/release`

**请求方式：** `GET`

**请求参数（Query Parameters）：**
- `gateway`：Gateway 名称
- `ip_addr`：Gateway 的 IP 地址
- `last_version`：上次获取的配置版本号（用于增量更新，首次为空）
- `supportPriorityConfig`：如果支持优先级配置，值为 `"1"`（可选）
- `lastPriorityVersions`：优先级配置的版本信息，格式为 `key=version`（可能有多个，可选）

**响应状态码：**
- `200 OK`：返回最新配置
- `304 Not Modified`：配置未更新（基于 `last_version` 判断）

**响应体格式（JSON）：**
```json
{
  "config": "{...}",  // Gateway 配置的 JSON 字符串
  "version": "v1.0.0",  // 配置版本号
  "priorityConfigs": [  // 优先级配置列表（可选）
    {
      "key": "canary",
      "config": "{...}",  // 优先级配置的 JSON 字符串
      "version": "v1.0.1"
    }
  ]
}
```

**使用场景：**
- Gateway 启动时拉取初始配置
- 每 5 秒轮询检查配置更新
- 支持增量更新（通过 `last_version` 判断是否需要更新）

**调用示例：**
```bash
# 首次请求（无 last_version）
GET http://control-service:8000/v1/control/gateway/release?gateway=my-gateway&ip_addr=192.168.1.100

# 增量更新请求（带 last_version）
GET http://control-service:8000/v1/control/gateway/release?gateway=my-gateway&ip_addr=192.168.1.100&last_version=v1.0.0&supportPriorityConfig=1&lastPriorityVersions=canary=v1.0.1
```

#### 2. 功能开关 API：`/v1/control/gateway/features`

**请求方式：** `GET`

**请求参数（Query Parameters）：**
- `gateway`：Gateway 名称
- `ip_addr`：Gateway 的 IP 地址

**响应状态码：**
- `200 OK`：返回功能开关配置
- `304 Not Modified`：功能开关未更新

**响应体格式（JSON）：**
```json
{
  "gateway": "gateway-name",
  "features": {
    "gw:PriorityConfig": true,  // 功能开关名称 -> 是否启用
    "feature:name": false
  }
}
```

**使用场景：**
- Gateway 启动时加载功能开关
- 每 5 秒轮询检查功能开关更新
- 动态启用/禁用 Gateway 功能

**调用示例：**
```bash
GET http://control-service:8000/v1/control/gateway/features?gateway=my-gateway&ip_addr=192.168.1.100
```

### 控制服务需要实现的功能

控制服务需要：

1. **存储和管理 Gateway 配置**
   - 支持按 Gateway 名称和 IP 地址分发配置
   - 支持配置版本管理
   - 支持优先级配置（用于灰度发布）

2. **实现配置分发逻辑**
   - 根据 `gateway` 和 `ip_addr` 返回对应配置
   - 支持 `last_version` 实现增量更新（返回 304）
   - 支持优先级配置的版本管理

3. **实现功能开关管理**
   - 支持按 Gateway 设置功能开关
   - 返回功能开关的启用/禁用状态

4. **支持多实例高可用**
   - 支持多个控制服务地址（用逗号分隔）
   - Gateway 会自动轮询和故障转移

### 工作流程

```
启动时：
1. Gateway 检查是否配置了控制服务
2. 从控制服务拉取初始配置 → 写入本地文件
3. 从控制服务拉取功能开关
4. 启动后台轮询（每5秒一次）

运行时：
1. 后台协程定期轮询控制服务
2. 如果配置有更新 → 写入本地文件
3. 本地文件监听器检测到变化 → 触发配置重载
4. Gateway 应用新配置（无需重启）
```

### 优先级配置（灰度发布）

优先级配置支持灰度发布场景：

- 控制服务可以返回多个优先级配置（如 `canary.yaml`、`staging.yaml`）
- 优先级配置会覆盖主配置中的同名 endpoint
- 支持按版本管理优先级配置
- Gateway 会自动清理过期的优先级配置

### 故障处理

- 如果控制服务不可用，Gateway 会使用本地配置文件作为后备
- 支持多个控制服务地址，自动故障转移
- 配置拉取失败不会中断 Gateway 运行
