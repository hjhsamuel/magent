# magent

magent 是运行在 Kubernetes 中的 Agent 会话服务。客户端通过 HTTP 创建用户工作区、发送消息和提交审批，通过 SSE 接收同一会话的增量回复。

## Kubernetes 部署

1. 准备支持事务的 MongoDB、标题生成 API，以及集群可拉取的 magent 和 Codex app-server 镜像。magent 镜像使用仓库根目录的 Dockerfile 构建。
2. 在工作区节点的可写目录 `/workspace` 中放入 `config.toml`、`models.json` 和 `skills/`。将 PV 的 `kubernetes.io/hostname` 设为该节点的主机名。
3. 在 [k8s.yaml](k8s.yaml) 中填写镜像地址和下表必填变量，将 `MAGENT_RUNTIME_IMAGE` 改为 `MAGENT_RUNTIME_POD_IMAGE`，并将无头 Service `agent` 的 selector 改为 `agent: "true"` 以匹配用户 Pod。保留单副本，因为 SSE 缓存位于进程内。

```sh
kubectl label node <工作区节点> agent=true --overwrite
kubectl apply -f k8s.yaml
kubectl -n agent rollout status deployment/magent
```

部署后可访问 `http://<节点地址>:31000/magent/swagger/index.html` 查看接口。

## 环境变量

启动时读取进程环境变量，也支持从当前工作目录的 `.env` 补充配置。下表标为「必填」的变量若为空，服务会拒绝启动。

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `MAGENT_SERVER_HOST` | `0.0.0.0` | HTTP 监听地址 |
| `MAGENT_SERVER_PORT` | `8080` | HTTP 端口，修改时同步 Service 的 targetPort |
| `MAGENT_SERVER_HEADER` | 必填 | 保存用户 JWT 的 **Cookie 名** |
| `MAGENT_SERVER_SALT` | 必填 | JWT 签名密钥，须与签发方一致 |
| `MAGENT_STORAGE_DB` | 必填 | MongoDB 数据库名，清单示例为 `magent` |
| `MAGENT_STORAGE_DSN` | 必填 | MongoDB 连接 URI |
| `MAGENT_STORAGE_MAXIDLECONN` | `200` | MongoDB 连接池最大连接数 |
| `MAGENT_STORAGE_MINIDLECONN` | `5` | MongoDB 连接池最小连接数 |
| `MAGENT_STORAGE_IDLETIME` | `5m` | 连接最大空闲时长，使用 Go duration 格式 |
| `MAGENT_STORAGE_DEBUG` | `false` | 开启 MongoDB 命令调试日志 |
| `MAGENT_RUNTIME_TOKEN` | 必填 | magent 连接 Codex WebSocket 的令牌 |
| `MAGENT_RUNTIME_TOPIC_URL` | 必填 | 兼容 OpenAI Chat Completions 的 API 基础地址 |
| `MAGENT_RUNTIME_TOPIC_KEY` | 必填 | 标题生成 API 密钥 |
| `MAGENT_RUNTIME_TOPIC_MODEL` | 必填 | 标题生成模型名 |
| `MAGENT_RUNTIME_POD_IMAGE` | `codex:latest` | 用户 Pod 镜像 |
| `MAGENT_RUNTIME_POD_LABEL` | `agent=true` | 用户 Pod 标签，同时用于节点选择，格式为 `key=value` |
| `MAGENT_RUNTIME_POD_SUBDOM` | `agent` | 用户 Pod 子域，对应无头 Service 名 |
| `MAGENT_RUNTIME_POD_PVC` | `agent-workspace` | 用户 Pod 使用的 PVC，与服务共享工作区 |
| `MAGENT_LOG_PATH` | `logs/magent.log` | 日志文件，容器内默认在 `/app/logs/` |
| `MAGENT_LOG_LEVEL` | `info` | Logrus 日志级别 |
| `MAGENT_LOG_SIZE` | `50` | 单个日志文件的轮转阈值（MB） |
| `MAGENT_LOG_BACKUP` | `3` | 保留的轮转日志文件数 |

## 发起会话

以下请求共用前缀 `/magent/v1`，并使用同一用户的 JWT。服务优先读取配置的 Cookie，其次读取 `Authorization: Bearer <JWT>`。JWT 需要包含非零数值 `id` 和非空字符串 `name`。

1. 调用 `POST /workspace` 创建当前用户的工作区，通过 `GET /workspace` 查询，等待 `data.status` 为 `running`。
2. 打开 `GET /session/stream` 创建会话并保持连接，从 `init` 事件的 JSON 字段 `data` 取得会话 ID。命令行可用以下方式观察原始事件。

   ```sh
   curl -N -H "Authorization: Bearer $JWT" "$BASE_URL/magent/v1/session/stream"
   ```

3. 在另一个请求中向 `POST /session/<会话 ID>/chat` 提交 JSON，如 `{"content":"你好"}`。HTTP 响应确认提交，回复从已打开的 SSE 连接返回。

## SSE 消息格式

响应类型为 `text/event-stream`。每帧以空行结束，`event` 指定事件类型，`id` 是流内序号，`data` 按事件类型解析。以下是节选的原始帧。

```text
id:0
event:init
data:{"data":"674000000000000000000001"}

id:1
event:message_delta
data:{"data":"你好"}

id:3
event:complete
data:你好

```

| event | data 的内容 | 客户端处理 |
| --- | --- | --- |
| `init` | `{"data":"<会话 ID>"}` | 保存会话 ID，供发送消息及重连使用 |
| `title` | `{"data":"会话标题"}` | 更新会话名称 |
| `reasoning_delta` | `{"data":"思考片段"}` | 按序追加到思考区域 |
| `message_delta` | `{"data":"正文片段"}` | 按序追加到当前回复 |
| `part_end` | 无业务载荷 | 标记当前输出项结束，忽略 data |
| `approve` | 包含 `id`、`method`、`content` 的 JSON 对象 | 展示请求并回传决定。权限请求另带 `permission`，MCP 表单请求另带 `schema` |
| `error` | `{"message":"错误说明","additional_details":"补充信息"}` | 展示运行时错误 |
| `complete` | 原始文本，内容为最终回复或本轮错误信息 | 结束当前轮次，保留订阅以接收后续轮次 |

审批事件的 JSON `id` 是运行时请求 ID。向 `POST /session/<会话 ID>/approval` 回传原始类型的请求 ID 与原始方法名。例如，拒绝一次命令执行请求可提交以下 JSON。

```json
{"accept":false,"info":{"id":7,"method":"item/commandExecution/requestApproval"}}
```

**断线续传**：使用 `GET /session/stream?id=<会话 ID>&seq=<最后已处理的正序号>`。普通事件从 1 递增；`init` 和重连时补发的待审批请求使用 0，应保留已有游标，并按审批请求 ID 去重。每条流同时支持一个订阅者，最多缓存 1024 个事件。省略 `seq` 或传 0 会读取所有保留事件；游标越界时从最近保留的 `part_end` 或 `complete` 恢复，若无标记则从最早保留事件开始。生产者和订阅者均断开 10 秒后缓存会被回收，历史轮次可通过 `GET /session/<会话 ID>/turns?limit=10` 查询。

