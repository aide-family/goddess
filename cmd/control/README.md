# Control Service

控制服务用于集中管理 Gateway 配置，支持动态配置更新和功能开关管理。

## 启动控制服务

```bash
./goddess control --http.addr=0.0.0.0:8000 --data.dir=./data/control
```

**参数说明：**
- `--http.addr`：HTTP 服务监听地址（默认：`:8000`）
- `--data.dir`：配置数据存储目录（默认：`./data/control`）

## 数据目录结构

控制服务使用文件系统存储配置，目录结构如下：

```
data/control/
├── {gateway-name}/
│   ├── config.yaml          # Gateway 主配置
│   ├── version.txt           # 配置版本号
│   ├── features.json         # 功能开关配置（可选）
│   └── priority/             # 优先级配置目录（可选）
│       ├── canary.yaml       # 金丝雀配置
│       ├── canary.version.txt
│       └── staging.yaml      # 预发布配置
│           └── staging.version.txt
```

## 配置示例

### 1. 创建 Gateway 配置

在 `data/control/my-gateway/config.yaml` 中创建配置：

```yaml
name: my-gateway
version: v1
endpoints:
  - path: /api/*
    protocol: HTTP
    backends:
      - target: 127.0.0.1:8000
middlewares:
  - name: logging
  - name: cors
```

### 2. 设置配置版本

在 `data/control/my-gateway/version.txt` 中写入版本号：

```
v1.0.0
```

### 3. 配置功能开关（可选）

在 `data/control/my-gateway/features.json` 中配置：

```json
{
  "features": {
    "gw:PriorityConfig": true,
    "feature:new-feature": false
  }
}
```

### 4. 配置优先级配置（可选）

在 `data/control/my-gateway/priority/canary.yaml` 中创建金丝雀配置：

```yaml
name: canary
version: v1
endpoints:
  - path: /api/v2/*
    protocol: HTTP
    backends:
      - target: 127.0.0.1:8001
```

在 `data/control/my-gateway/priority/canary.version.txt` 中写入版本号：

```
v1.0.1
```

## API 使用

### 1. 获取 Gateway 配置

```bash
curl "http://localhost:8000/v1/control/gateway/release?gateway=my-gateway&ip_addr=192.168.1.100"
```

**响应示例：**
```json
{
  "config": "{...}",
  "version": "v1.0.0",
  "priorityConfigs": [
    {
      "key": "canary",
      "config": "{...}",
      "version": "v1.0.1"
    }
  ]
}
```

### 2. 增量更新（带版本号）

```bash
curl "http://localhost:8000/v1/control/gateway/release?gateway=my-gateway&ip_addr=192.168.1.100&last_version=v1.0.0"
```

如果配置未更新，返回 `304 Not Modified`。

### 3. 获取功能开关

```bash
curl "http://localhost:8000/v1/control/gateway/features?gateway=my-gateway&ip_addr=192.168.1.100"
```

**响应示例：**
```json
{
  "gateway": "my-gateway",
  "features": {
    "gw:PriorityConfig": true,
    "feature:new-feature": false
  }
}
```

## Gateway 连接控制服务

在 Gateway 启动时指定控制服务：

```bash
./goddess gateway \
  --ctrl.name=my-gateway \
  --ctrl.service=http://localhost:8000 \
  --conf.priority=./canary
```

Gateway 会自动从控制服务拉取配置并定期更新。

## 更新配置

要更新 Gateway 配置：

1. 修改 `data/control/{gateway-name}/config.yaml`
2. 更新 `data/control/{gateway-name}/version.txt` 中的版本号
3. Gateway 会在下次轮询（5秒内）时自动检测到变化并应用新配置

## 注意事项

- 配置文件的格式必须符合 Gateway 配置的 proto 定义
- 版本号用于增量更新，每次更新配置时应该递增版本号
- 优先级配置会覆盖主配置中的同名 endpoint
- 控制服务支持热更新，修改配置文件后无需重启服务
