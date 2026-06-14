# GoClaw HTTP / WebSocket / MCP 接口完整分析

> 项目共 **~230 条 HTTP 路由** + **~130 个 WebSocket RPC 方法** + **~50 个 MCP 工具**  
> 本文档覆盖所有接口的路径、方法、用途、认证要求，以及协议字段与枚举值

---

## 1. 三大接口体系总览

| 接口体系 | 数量 | 协议 | 用途 |
|----------|------|------|------|
| **HTTP REST** | ~230 条路由 | REST/JSON | 管理面 API（CRUD、配置、查询） |
| **WebSocket RPC** | ~130 个方法 | 帧协议 v3 (JSON-RPC 2.0) | 实时交互面（聊天、事件推送、状态同步） |
| **MCP 工具** | ~50 个工具 | MCP (JSON-RPC 2.0) | LLM 工具调用（Agent 执行时使用） |

---

## 2. WebSocket 帧协议 v3

### 2.1 三种帧类型

| 帧类型 | 方向 | 常量 | 结构 |
|--------|------|------|------|
| `req` | Client→Server | `FrameTypeRequest` | `{ type:"req", id:string, method:string, params?:json }` |
| `res` | Server→Client | `FrameTypeResponse` | `{ type:"res", id:string, ok:boolean, payload?:any, error?:ErrorShape }` |
| `event` | Server→Client | `FrameTypeEvent` | `{ type:"event", event:string, payload?:any, seq?:int64, stateVersion?:StateVersion }` |

### 2.2 字段说明

**RequestFrame**:
- `type`: 固定 `"req"`
- `id`: 客户端生成的唯一请求 ID（用于匹配响应）
- `method`: RPC 方法名（如 `"chat.send"`）
- `params`: 方法参数（JSON）

**ResponseFrame**:
- `type`: 固定 `"res"`
- `id`: 与请求匹配的 ID
- `ok`: `true` 成功 / `false` 失败
- `payload`: 成功时的返回数据
- `error`: 失败时的错误信息

**EventFrame**:
- `type`: 固定 `"event"`
- `event`: 事件名（如 `"chat"` / `"agent"` / `"team.task.created"`）
- `payload`: 事件数据
- `seq`: 有序序列号（用于客户端排序）
- `stateVersion`: 状态版本计数器（用于乐观同步）

**StateVersion**:
```json
{ "presence": 12345, "health": 67890 }
```

### 2.3 错误码枚举

| 常量 | 值 | 说明 |
|------|-----|------|
| `ErrInvalidRequest` | `INVALID_REQUEST` | 请求格式错误 |
| `ErrUnavailable` | `UNAVAILABLE` | 服务不可用 |
| `ErrNotLinked` | `NOT_LINKED` | 未链接 |
| `ErrNotPaired` | `NOT_PAIRED` | 未配对 |
| `ErrAgentTimeout` | `AGENT_TIMEOUT` | Agent 执行超时 |
| `ErrUnauthorized` | `UNAUTHORIZED` | 未授权 |
| `ErrNotFound` | `NOT_FOUND` | 资源不存在 |
| `ErrAlreadyExists` | `ALREADY_EXISTS` | 资源已存在 |
| `ErrResourceExhausted` | `RESOURCE_EXHAUSTED` | 资源耗尽 |
| `ErrFailedPrecondition` | `FAILED_PRECONDITION` | 前置条件不满足 |
| `ErrInternal` | `INTERNAL` | 内部错误 |
| `ErrTenantAccessRevoked` | `TENANT_ACCESS_REVOKED` | 租户访问已撤销 |
| `ErrNotImplemented` | `NOT_IMPLEMENTED` | 未实现 |

### 2.4 角色权限枚举

| 角色 | 值 | 等级 | 说明 |
|------|-----|------|------|
| `RoleOwner` | `"owner"` | 4 | 租户管理 + 全部权限 |
| `RoleAdmin` | `"admin"` | 3 | 全部管理操作 |
| `RoleOperator` | `"operator"` | 2 | 读写（无管理操作） |
| `RoleViewer` | `"viewer"` | 1 | 只读 |
| `RoleNone` | `""` | 0 | 哨兵值（未分类方法拒绝所有角色） |

---

## 3. WebSocket 方法完整列表

### 3.1 核心聊天

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `chat.send` | `MethodChatSend` | operator | 发送消息给 Agent |
| `chat.history` | `MethodChatHistory` | viewer | 获取会话历史 |
| `chat.abort` | `MethodChatAbort` | operator | 中止 Agent 执行 |
| `chat.inject` | `MethodChatInject` | admin | 注入系统消息 |
| `chat.session.status` | `MethodChatSessionStatus` | viewer | 获取会话状态 |

### 3.2 Agent 管理

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `agent` | `MethodAgent` | operator | 发送消息并等待完成 |
| `agent.wait` | `MethodAgentWait` | operator | 发送消息并等待（阻塞） |
| `agent.identity.get` | `MethodAgentIdentityGet` | viewer | 获取 Agent 身份信息 |
| `agents.list` | `MethodAgentsList` | viewer | 列出所有 Agent |
| `agents.create` | `MethodAgentsCreate` | admin | 创建 Agent |
| `agents.update` | `MethodAgentsUpdate` | admin | 更新 Agent |
| `agents.delete` | `MethodAgentsDelete` | admin | 删除 Agent |
| `agents.files.list` | `MethodAgentsFileList` | viewer | 列出 Agent 上下文文件 |
| `agents.files.get` | `MethodAgentsFileGet` | viewer | 获取文件内容 |
| `agents.files.set` | `MethodAgentsFileSet` | admin | 设置文件内容 |

### 3.3 会话管理

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `sessions.list` | `MethodSessionsList` | viewer | 列出会话 |
| `sessions.preview` | `MethodSessionsPreview` | viewer | 会话预览 |
| `sessions.patch` | `MethodSessionsPatch` | operator | 更新会话元数据 |
| `sessions.delete` | `MethodSessionsDelete` | admin | 删除会话 |
| `sessions.reset` | `MethodSessionsReset` | operator | 重置会话 |
| `sessions.compact` | `MethodSessionsCompact` | operator | 压缩会话历史 |

### 3.4 配置管理

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `config.get` | `MethodConfigGet` | admin | 获取配置 |
| `config.apply` | `MethodConfigApply` | admin | 应用完整配置 |
| `config.patch` | `MethodConfigPatch` | admin | 部分更新配置 |
| `config.schema` | `MethodConfigSchema` | admin | 获取配置 Schema |
| `config.defaults` | `MethodConfigDefaults` | admin | 获取默认配置 |

### 3.5 技能管理

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `skills.list` | `MethodSkillsList` | viewer | 列出技能 |
| `skills.get` | `MethodSkillsGet` | viewer | 获取技能详情 |
| `skills.update` | `MethodSkillsUpdate` | admin | 更新技能 |

### 3.6 定时任务

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `cron.list` | `MethodCronList` | viewer | 列出定时任务 |
| `cron.create` | `MethodCronCreate` | operator | 创建定时任务 |
| `cron.update` | `MethodCronUpdate` | operator | 更新定时任务 |
| `cron.delete` | `MethodCronDelete` | operator | 删除定时任务 |
| `cron.toggle` | `MethodCronToggle` | operator | 启用/禁用 |
| `cron.status` | `MethodCronStatus` | viewer | 查看状态 |
| `cron.run` | `MethodCronRun` | operator | 立即执行 |
| `cron.runs` | `MethodCronRuns` | viewer | 查看执行记录 |

### 3.7 通道管理

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `channels.list` | `MethodChannelsList` | viewer | 列出通道 |
| `channels.status` | `MethodChannelsStatus` | viewer | 通道状态 |
| `channels.toggle` | `MethodChannelsToggle` | admin | 启用/禁用通道 |
| `channels.instances.list` | `MethodChannelInstancesList` | viewer | 列出通道实例 |
| `channels.instances.get` | `MethodChannelInstancesGet` | viewer | 获取实例详情 |
| `channels.instances.create` | `MethodChannelInstancesCreate` | admin | 创建实例 |
| `channels.instances.update` | `MethodChannelInstancesUpdate` | admin | 更新实例 |
| `channels.instances.delete` | `MethodChannelInstancesDelete` | admin | 删除实例 |

### 3.8 设备配对

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `device.pair.request` | `MethodPairingRequest` | operator | 发起配对 |
| `device.pair.approve` | `MethodPairingApprove` | operator | 审批配对 |
| `device.pair.deny` | `MethodPairingDeny` | operator | 拒绝配对 |
| `device.pair.list` | `MethodPairingList` | viewer | 列出已配对设备 |
| `device.pair.revoke` | `MethodPairingRevoke` | operator | 撤销配对 |

### 3.9 执行审批

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `exec.approval.list` | `MethodApprovalsList` | viewer | 列出待审批 |
| `exec.approval.approve` | `MethodApprovalsApprove` | operator | 批准执行 |
| `exec.approval.deny` | `MethodApprovalsDeny` | operator | 拒绝执行 |

### 3.10 心跳

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `heartbeat.get` | `MethodHeartbeatGet` | viewer | 获取心跳配置 |
| `heartbeat.set` | `MethodHeartbeatSet` | admin | 设置心跳 |
| `heartbeat.toggle` | `MethodHeartbeatToggle` | admin | 启用/禁用 |
| `heartbeat.test` | `MethodHeartbeatTest` | admin | 测试心跳 |
| `heartbeat.logs` | `MethodHeartbeatLogs` | viewer | 查看日志 |
| `heartbeat.checklist.get` | `MethodHeartbeatChecklistGet` | viewer | 获取检查清单 |
| `heartbeat.checklist.set` | `MethodHeartbeatChecklistSet` | admin | 设置检查清单 |
| `heartbeat.targets` | `MethodHeartbeatTargets` | viewer | 查看目标 |

### 3.11 配置权限

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `config.permissions.list` | `MethodConfigPermissionsList` | viewer | 列出权限 |
| `config.permissions.check` | `MethodConfigPermissionsCheck` | viewer | 检查权限 |
| `config.permissions.grant` | `MethodConfigPermissionsGrant` | admin | 授权 |
| `config.permissions.revoke` | `MethodConfigPermissionsRevoke` | admin | 撤销 |

### 3.12 团队协作

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `teams.list` | `MethodTeamsList` | viewer | 列出团队 |
| `teams.create` | `MethodTeamsCreate` | admin | 创建团队 |
| `teams.get` | `MethodTeamsGet` | viewer | 获取团队 |
| `teams.delete` | `MethodTeamsDelete` | admin | 删除团队 |
| `teams.update` | `MethodTeamsUpdate` | admin | 更新团队 |
| `teams.tasks.list` | `MethodTeamsTaskList` | viewer | 列出任务 |
| `teams.tasks.get` | `MethodTeamsTaskGet` | viewer | 获取任务 |
| `teams.tasks.create` | `MethodTeamsTaskCreate` | operator | 创建任务 |
| `teams.tasks.delete` | `MethodTeamsTaskDelete` | operator | 删除任务 |
| `teams.tasks.assign` | `MethodTeamsTaskAssign` | operator | 分配任务 |
| `teams.tasks.approve` | `MethodTeamsTaskApprove` | operator | 批准任务 |
| `teams.tasks.reject` | `MethodTeamsTaskReject` | operator | 拒绝任务 |
| `teams.tasks.comment` | `MethodTeamsTaskComment` | operator | 评论任务 |
| `teams.tasks.comments` | `MethodTeamsTaskComments` | viewer | 查看评论 |
| `teams.tasks.events` | `MethodTeamsTaskEvents` | viewer | 查看事件 |
| `teams.members.add` | `MethodTeamsMembersAdd` | admin | 添加成员 |
| `teams.members.remove` | `MethodTeamsMembersRemove` | admin | 移除成员 |
| `teams.known_users` | `MethodTeamsKnownUsers` | viewer | 已知用户 |
| `teams.scopes` | `MethodTeamsScopes` | viewer | 可见范围 |
| `teams.workspace.list` | `MethodTeamsWorkspaceList` | viewer | 工作区文件列表 |
| `teams.workspace.read` | `MethodTeamsWorkspaceRead` | viewer | 读取工作区文件 |
| `teams.workspace.delete` | `MethodTeamsWorkspaceDelete` | operator | 删除工作区文件 |
| `teams.events.list` | `MethodTeamsEventsList` | viewer | 团队事件列表 |

### 3.13 Agent 链接（委派）

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `agents.links.list` | `MethodAgentsLinksList` | viewer | 列出链接 |
| `agents.links.create` | `MethodAgentsLinksCreate` | admin | 创建链接 |
| `agents.links.update` | `MethodAgentsLinksUpdate` | admin | 更新链接 |
| `agents.links.delete` | `MethodAgentsLinksDelete` | admin | 删除链接 |

### 3.14 Hook

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `hooks.list` | `MethodHooksList` | viewer | 列出 Hook |
| `hooks.create` | `MethodHooksCreate` | admin | 创建 Hook |
| `hooks.update` | `MethodHooksUpdate` | admin | 更新 Hook |
| `hooks.delete` | `MethodHooksDelete` | admin | 删除 Hook |
| `hooks.toggle` | `MethodHooksToggle` | admin | 启用/禁用 |
| `hooks.test` | `MethodHooksTest` | operator | 测试 Hook |
| `hooks.history` | `MethodHooksHistory` | viewer | 执行历史 |

### 3.15 API Key

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `api_keys.list` | `MethodAPIKeysList` | viewer | 列出 API Key |
| `api_keys.create` | `MethodAPIKeysCreate` | admin | 创建 API Key |
| `api_keys.revoke` | `MethodAPIKeysRevoke` | admin | 撤销 API Key |

### 3.16 工作站

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `workstations.list` | `MethodWorkstationsList` | viewer | 列出工作站 |
| `workstations.get` | `MethodWorkstationsGet` | viewer | 获取工作站 |
| `workstations.create` | `MethodWorkstationsCreate` | admin | 创建工作站 |
| `workstations.update` | `MethodWorkstationsUpdate` | admin | 更新工作站 |
| `workstations.delete` | `MethodWorkstationsDelete` | admin | 删除工作站 |
| `workstations.testConnection` | `MethodWorkstationsTest` | admin | 测试连接 |
| `workstations.linkAgent` | `MethodWorkstationsLinkAgent` | admin | 绑定 Agent |
| `workstations.unlinkAgent` | `MethodWorkstationsUnlinkAgent` | admin | 解绑 Agent |
| `workstations.permissions.list` | `MethodWorkstationsPermList` | viewer | 列出权限 |
| `workstations.permissions.add` | `MethodWorkstationsPermAdd` | admin | 添加权限 |
| `workstations.permissions.remove` | `MethodWorkstationsPermRemove` | admin | 移除权限 |
| `workstations.permissions.toggle` | `MethodWorkstationsPermToggle` | admin | 启用/禁用 |
| `workstations.activity.list` | `MethodWorkstationsListActivity` | viewer | 操作审计 |

### 3.17 语音/TTS

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `voices.list` | `MethodVoicesList` | viewer | 列出语音 |
| `voices.refresh` | `MethodVoicesRefresh` | admin | 刷新语音列表 |
| `tts.status` | `MethodTTSStatus` | viewer | TTS 状态 |
| `tts.enable` | `MethodTTSEnable` | admin | 启用 TTS |
| `tts.disable` | `MethodTTSDisable` | admin | 禁用 TTS |
| `tts.convert` | `MethodTTSConvert` | operator | 文字转语音 |
| `tts.setProvider` | `MethodTTSSetProvider` | admin | 设置 TTS Provider |
| `tts.providers` | `MethodTTSProviders` | viewer | 列出 TTS Provider |

### 3.18 浏览器

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `browser.act` | `MethodBrowserAct` | operator | 执行浏览器操作 |
| `browser.snapshot` | `MethodBrowserSnapshot` | operator | 获取页面快照 |
| `browser.screenshot` | `MethodBrowserScreenshot` | operator | 截图 |

### 3.19 系统

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `connect` | `MethodConnect` | viewer | 握手连接 |
| `health` | `MethodHealth` | viewer | 健康检查 |
| `status` | `MethodStatus` | viewer | 系统状态 |
| `usage.get` | `MethodUsageGet` | viewer | 使用量查询 |
| `usage.summary` | `MethodUsageSummary` | viewer | 使用量摘要 |
| `quota.usage` | `MethodQuotaUsage` | viewer | 配额使用 |
| `logs.tail` | `MethodLogsTail` | admin | 实时日志 |
| `send` | `MethodSend` | operator | 发送消息到通道 |

### 3.20 Bitrix24 / Zalo / WhatsApp

| 方法 | 常量 | 权限 | 用途 |
|------|------|------|------|
| `bitrix.portals.list` | `MethodBitrixPortalsList` | viewer | 列出 Bitrix24 门户 |
| `bitrix.portals.create` | `MethodBitrixPortalsCreate` | admin | 创建门户 |
| `bitrix.portals.get_install_url` | `MethodBitrixPortalsGetInstallURL` | admin | 获取安装链接 |
| `bitrix.portals.delete` | `MethodBitrixPortalsDelete` | admin | 删除门户 |
| `zalo.personal.qr.start` | `MethodZaloPersonalQRStart` | operator | Zalo QR 登录 |
| `zalo.personal.contacts` | `MethodZaloPersonalContacts` | operator | Zalo 联系人 |
| `whatsapp.qr.start` | `MethodWhatsAppQRStart` | operator | WhatsApp QR 登录 |

---

## 4. WebSocket 事件推送

### 4.1 事件名完整列表

| 事件名 | 常量 | 分类 | payload.type | 说明 |
|--------|------|------|-------------|------|
| `agent` | `EventAgent` | Agent | run.started / run.completed / run.failed / run.cancelled / run.retrying / tool.call / tool.result / block.reply / activity | Agent 运行生命周期 |
| `chat` | `EventChat` | 聊天 | chunk / message / thinking | 聊天流式事件 |
| `team.task.created` | `EventTeamTaskCreated` | 团队任务 | — | 任务创建 |
| `team.task.completed` | `EventTeamTaskCompleted` | 团队任务 | — | 任务完成 |
| `team.task.claimed` | `EventTeamTaskClaimed` | 团队任务 | — | 任务被认领 |
| `team.task.cancelled` | `EventTeamTaskCancelled` | 团队任务 | — | 任务取消 |
| `team.task.failed` | `EventTeamTaskFailed` | 团队任务 | — | 任务失败 |
| `team.task.reviewed` | `EventTeamTaskReviewed` | 团队任务 | — | 任务审核 |
| `team.task.approved` | `EventTeamTaskApproved` | 团队任务 | — | 任务批准 |
| `team.task.rejected` | `EventTeamTaskRejected` | 团队任务 | — | 任务拒绝 |
| `team.task.progress` | `EventTeamTaskProgress` | 团队任务 | — | 任务进度 |
| `team.task.commented` | `EventTeamTaskCommented` | 团队任务 | — | 任务评论 |
| `team.task.assigned` | `EventTeamTaskAssigned` | 团队任务 | — | 任务分配 |
| `team.task.dispatched` | `EventTeamTaskDispatched` | 团队任务 | — | 任务分发 |
| `team.task.updated` | `EventTeamTaskUpdated` | 团队任务 | — | 任务更新 |
| `team.task.deleted` | `EventTeamTaskDeleted` | 团队任务 | — | 任务删除 |
| `team.task.stale` | `EventTeamTaskStale` | 团队任务 | — | 任务过期 |
| `team.task.attachment_added` | `EventTeamTaskAttachmentAdded` | 团队任务 | — | 附件添加 |
| `team.message.sent` | `EventTeamMessageSent` | 团队消息 | — | 团队消息发送 |
| `team.created` | `EventTeamCreated` | 团队 CRUD | — | 团队创建 |
| `team.updated` | `EventTeamUpdated` | 团队 CRUD | — | 团队更新 |
| `team.deleted` | `EventTeamDeleted` | 团队 CRUD | — | 团队删除 |
| `team.member.added` | `EventTeamMemberAdded` | 团队 CRUD | — | 成员添加 |
| `team.member.removed` | `EventTeamMemberRemoved` | 团队 CRUD | — | 成员移除 |
| `team.leader.processing` | `EventTeamLeaderProcessing` | 团队 | — | Leader 开始处理结果 |
| `delegation.started` | `EventDelegationStarted` | 委派 | — | 委派开始 |
| `delegation.completed` | `EventDelegationCompleted` | 委派 | — | 委派完成 |
| `delegation.failed` | `EventDelegationFailed` | 委派 | — | 委派失败 |
| `delegation.cancelled` | `EventDelegationCancelled` | 委派 | — | 委派取消 |
| `delegation.progress` | `EventDelegationProgress` | 委派 | — | 委派进度 |
| `delegation.accumulated` | `EventDelegationAccumulated` | 委派 | — | 结果累积（兄弟还在跑） |
| `delegation.announce` | `EventDelegationAnnounce` | 委派 | — | 全部完成，结果通告 |
| `workspace.file.changed` | `EventWorkspaceFileChanged` | 工作区 | — | 文件变更 |
| `agent_link.created` | `EventAgentLinkCreated` | 链接 | — | Agent 链接创建 |
| `agent_link.updated` | `EventAgentLinkUpdated` | 链接 | — | Agent 链接更新 |
| `agent_link.deleted` | `EventAgentLinkDeleted` | 链接 | — | Agent 链接删除 |
| `trace.updated` | `EventTraceUpdated` | 追踪 | — | 追踪更新 |
| `trace.status` | `EventTraceStatusChanged` | 追踪 | — | 追踪状态变更 |
| `skill.deps.checked` | `EventSkillDepsChecked` | 技能 | — | 依赖检查完成 |
| `skill.deps.complete` | `EventSkillDepsComplete` | 技能 | — | 依赖处理完成 |
| `skill.deps.installing` | `EventSkillDepsInstalling` | 技能 | — | 依赖安装中 |
| `skill.deps.installed` | `EventSkillDepsInstalled` | 技能 | — | 依赖安装完成 |
| `session.updated` | `EventSessionUpdated` | 会话 | — | 会话更新 |
| `exec.approval.requested` | `EventExecApprovalReq` | 审批 | — | 执行审批请求 |
| `exec.approval.resolved` | `EventExecApprovalRes` | 审批 | — | 执行审批结果 |
| `device.pair.requested` | `EventDevicePairReq` | 配对 | — | 设备配对请求 |
| `device.pair.resolved` | `EventDevicePairRes` | 配对 | — | 设备配对结果 |
| `node.pair.requested` | `EventNodePairRequested` | 配对 | — | 节点配对请求 |
| `node.pair.resolved` | `EventNodePairResolved` | 配对 | — | 节点配对结果 |
| `agent.summoning` | `EventAgentSummoning` | Agent | — | Agent 召唤中 |
| `presence` | `EventPresence` | 系统 | — | 在线状态 |
| `health` | `EventHealth` | 系统 | — | 健康状态 |
| `tick` | `EventTick` | 系统 | — | 心跳 |
| `shutdown` | `EventShutdown` | 系统 | — | 关闭通知 |
| `tenant.access.revoked` | `EventTenantAccessRevoked` | 安全 | — | 租户访问撤销 |
| `vault.enrich.progress` | `EventVaultEnrichProgress` | Vault | — | 文档富化进度 |
| `background.error` | `EventBackgroundError` | 错误 | — | 后台错误 |
| `workstation.exec.chunk` | `EventWorkstationExecChunk` | 工作站 | — | 远程执行输出 |
| `workstation.exec.done` | `EventWorkstationExecDone` | 工作站 | — | 远程执行完成 |
| `voicewake.changed` | `EventVoicewakeChanged` | 语音 | — | 唤醒词变更 |
| `talk.mode` | `EventTalkMode` | 语音 | — | 对话模式 |
| `connect.challenge` | `EventConnectChallenge` | 连接 | — | 连接挑战 |
| `cron` | `EventCron` | Cron | — | Cron 事件 |
| `heartbeat` | `EventHeartbeat` | 心跳 | — | 心跳事件 |
| `zalo.personal.qr.code` | `EventZaloPersonalQRCode` | Zalo | — | Zalo QR 码 |
| `zalo.personal.qr.done` | `EventZaloPersonalQRDone` | Zalo | — | Zalo QR 完成 |
| `whatsapp.qr.code` | `EventWhatsAppQRCode` | WhatsApp | — | WhatsApp QR 码 |
| `whatsapp.qr.done` | `EventWhatsAppQRDone` | WhatsApp | — | WhatsApp QR 完成 |

---

## 5. HTTP REST API 完整路由

### 5.1 Agent

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/agents` | auth | 列出 Agent |
| POST | `/v1/agents` | admin | 创建 Agent |
| GET | `/v1/agents/{id}` | auth | 获取 Agent |
| PUT | `/v1/agents/{id}` | admin | 更新 Agent |
| DELETE | `/v1/agents/{id}` | admin | 删除 Agent |
| POST | `/v1/agents/sync-workspace` | admin | 同步工作区 |
| GET | `/v1/agents/{id}/shares` | auth | 列出共享 |
| POST | `/v1/agents/{id}/shares` | admin | 共享 Agent |
| DELETE | `/v1/agents/{id}/shares/{userID}` | admin | 撤销共享 |
| POST | `/v1/agents/{id}/regenerate` | admin | 重新生成 |
| POST | `/v1/agents/{id}/resummon` | admin | 重新召唤 |
| POST | `/v1/agents/{id}/cancel-summon` | admin | 取消召唤 |
| GET | `/v1/agents/{id}/system-prompt-preview` | admin | 预览系统提示词 |
| GET | `/v1/agents/{id}/export/preview` | auth | 导出预览 |
| GET | `/v1/agents/{id}/export` | auth | 导出 |
| GET | `/v1/agents/{id}/export/download/{token}` | auth | 下载导出 |
| POST | `/v1/agents/import/preview` | admin | 导入预览 |
| POST | `/v1/agents/import` | admin | 导入 |
| POST | `/v1/agents/{id}/import` | admin | 合并导入 |
| GET | `/v1/agents/{id}/codex-pool-activity` | auth | Codex 池活动 |
| GET | `/v1/agents/{id}/instances` | auth | 列出用户实例 |
| GET | `/v1/agents/{id}/instances/{userID}/files` | auth | 获取实例文件 |
| PUT | `/v1/agents/{id}/instances/{userID}/files/{fileName}` | admin | 设置实例文件 |
| PATCH | `/v1/agents/{id}/instances/{userID}/metadata` | admin | 更新实例元数据 |

### 5.2 会话

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/sessions` | auth | 列出会话 |
| GET | `/v1/chat/sessions/{key}/history/follow` | auth | 追踪会话历史 |
| POST | `/v1/chat/sessions/{key}/branch` | auth | 分支会话 |

### 5.3 Provider

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/providers` | auth | 列出 Provider |
| POST | `/v1/providers` | auth | 创建 Provider |
| GET | `/v1/providers/{id}` | auth | 获取 Provider |
| PUT | `/v1/providers/{id}` | auth | 更新 Provider |
| DELETE | `/v1/providers/{id}` | auth | 删除 Provider |
| POST | `/v1/providers/{id}/verify` | auth | 验证连接 |
| POST | `/v1/providers/{id}/verify-embedding` | auth | 验证嵌入 |
| POST | `/v1/providers/{id}/reconnect` | auth | 重连 |
| GET | `/v1/providers/{id}/models` | auth | 列出模型 |
| GET | `/v1/providers/claude-cli/auth-status` | auth | Claude CLI 认证状态 |
| GET | `/v1/embedding/status` | auth | 嵌入状态 |
| GET | `/v1/model-pricing` | auth | 模型定价 |
| GET | `/v1/model-pricing/overrides` | auth | 定价覆盖 |
| PUT | `/v1/model-pricing/overrides` | auth | 设置定价覆盖 |
| DELETE | `/v1/model-pricing/overrides/{id}` | auth | 删除定价覆盖 |
| POST | `/v1/model-pricing/sync-openrouter` | master | 同步 OpenRouter 定价 |

### 5.4 技能

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/skills` | auth | 列出技能 |
| POST | `/v1/skills/upload` | admin | 上传技能 |
| POST | `/v1/skills/import` | admin | 导入技能 |
| GET | `/v1/skills/{id}` | auth | 获取技能 |
| PUT | `/v1/skills/{id}` | admin | 更新技能 |
| DELETE | `/v1/skills/{id}` | admin | 删除技能 |
| GET | `/v1/skills/{id}/files` | auth | 列出文件 |
| GET | `/v1/skills/{id}/files/{path...}` | auth | 读取文件 |
| GET | `/v1/skills/{id}/versions` | auth | 版本历史 |
| GET | `/v1/skills/{id}/grants/agent` | admin | Agent 授权列表 |
| POST | `/v1/skills/{id}/grants/agent` | admin | 授权 Agent |
| DELETE | `/v1/skills/{id}/grants/agent/{agentID}` | admin | 撤销 Agent |
| POST | `/v1/skills/{id}/grants/user` | admin | 授权用户 |
| DELETE | `/v1/skills/{id}/grants/user/{userID}` | admin | 撤销用户 |
| POST | `/v1/skills/{id}/toggle` | admin | 启用/禁用 |
| PUT | `/v1/skills/{id}/tenant-config` | admin | 设置租户配置 |
| DELETE | `/v1/skills/{id}/tenant-config` | admin | 删除租户配置 |
| POST | `/v1/skills/install-dep` | admin | 安装依赖 |
| POST | `/v1/skills/install-deps` | admin | 批量安装依赖 |
| POST | `/v1/skills/rescan-deps` | admin | 重新扫描依赖 |
| GET | `/v1/skills/runtimes` | admin | 运行时状态 |
| GET | `/v1/skills/export` | admin | 导出技能包 |
| GET | `/v1/skills/export/preview` | admin | 导出预览 |
| GET | `/v1/agents/{id}/skills` | auth | Agent 技能列表 |

### 5.5 MCP

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/mcp/servers` | auth | 列出服务器 |
| POST | `/v1/mcp/servers` | admin | 创建服务器 |
| GET | `/v1/mcp/servers/{id}` | auth | 获取服务器 |
| PUT | `/v1/mcp/servers/{id}` | admin | 更新服务器 |
| DELETE | `/v1/mcp/servers/{id}` | admin | 删除服务器 |
| POST | `/v1/mcp/servers/test` | admin | 测试连接 |
| POST | `/v1/mcp/servers/{id}/reconnect` | admin | 重连 |
| GET | `/v1/mcp/servers/{id}/tools` | auth | 列出工具 |
| GET | `/v1/mcp/servers/{id}/grants` | auth | 授权列表 |
| POST | `/v1/mcp/servers/{id}/grants/agent` | admin | 授权 Agent |
| DELETE | `/v1/mcp/servers/{id}/grants/agent/{agentID}` | admin | 撤销 Agent |
| POST | `/v1/mcp/servers/{id}/grants/user` | admin | 授权用户 |
| DELETE | `/v1/mcp/servers/{id}/grants/user/{userID}` | admin | 撤销用户 |
| GET | `/v1/mcp/grants/agent/{agentID}` | auth | Agent MCP 授权 |
| GET | `/v1/mcp/requests` | auth | 待审批请求 |
| POST | `/v1/mcp/requests` | auth | 创建请求 |
| POST | `/v1/mcp/requests/{id}/review` | admin | 审批请求 |
| GET | `/v1/mcp/servers/{id}/user-credentials` | auth | 用户凭证 |
| PUT | `/v1/mcp/servers/{id}/user-credentials` | auth | 设置凭证 |
| DELETE | `/v1/mcp/servers/{id}/user-credentials` | auth | 删除凭证 |
| GET | `/v1/mcp/export` | admin | 导出 MCP 配置 |
| GET | `/v1/mcp/export/preview` | admin | 导出预览 |
| POST | `/v1/mcp/import` | admin | 导入 MCP 配置 |

### 5.6 通道

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/channels/instances` | auth | 列出实例 |
| POST | `/v1/channels/instances` | admin | 创建实例 |
| GET | `/v1/channels/instances/{id}` | auth | 获取实例 |
| PUT | `/v1/channels/instances/{id}` | admin | 更新实例 |
| DELETE | `/v1/channels/instances/{id}` | admin | 删除实例 |
| GET | `/v1/channels/instances/{id}/writers` | auth | 列出写入者 |
| GET | `/v1/channels/instances/{id}/writers/groups` | auth | 写入者分组 |
| POST | `/v1/channels/instances/{id}/writers` | admin | 添加写入者 |
| POST | `/v1/channels/instances/{id}/writers/test` | auth | 测试写入者 |
| DELETE | `/v1/channels/instances/{id}/writers/{userId}` | admin | 移除写入者 |

### 5.7 联系人

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/contacts` | auth | 列出联系人 |
| GET | `/v1/contacts/resolve` | auth | 解析联系人 |
| POST | `/v1/contacts/merge` | admin | 合并联系人 |
| POST | `/v1/contacts/unmerge` | admin | 取消合并 |
| GET | `/v1/contacts/merged/{tenantUserId}` | auth | 已合并联系人 |
| GET | `/v1/tenant-users` | auth | 租户用户 |
| GET | `/v1/users/search` | auth | 搜索用户 |

### 5.8 记忆 & 知识图谱

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/agents/{agentID}/memory/documents` | auth | 文档列表 |
| GET | `/v1/agents/{agentID}/memory/documents/{path...}` | auth | 获取文档 |
| PUT | `/v1/agents/{agentID}/memory/documents/{path...}` | auth | 写入文档 |
| DELETE | `/v1/agents/{agentID}/memory/documents/{path...}` | auth | 删除文档 |
| GET | `/v1/agents/{agentID}/memory/chunks` | auth | 分块列表 |
| GET | `/v1/memory/documents` | auth | 全局文档列表 |
| GET | `/v1/agents/{agentID}/kg/entities` | auth | 实体列表 |
| GET | `/v1/agents/{agentID}/kg/entities/{entityID}` | auth | 获取实体 |
| DELETE | `/v1/agents/{agentID}/kg/entities/{entityID}` | auth | 删除实体 |
| GET | `/v1/agents/{agentID}/kg/graph` | auth | 图谱查询 |
| GET | `/v1/agents/{agentID}/kg/graph/compact` | auth | 紧凑图谱 |
| GET | `/v1/agents/{agentID}/kg/stats` | auth | 统计信息 |
| GET | `/v1/agents/{agentID}/kg/dedup` | auth | 去重候选 |
| GET | `/v1/agents/{agentID}/episodic` | auth | 情景记忆 |
| GET | `/v1/agents/{agentID}/evolution/metrics` | auth | 进化指标 |
| GET | `/v1/agents/{agentID}/evolution/suggestions` | auth | 进化建议 |
| PATCH | `/v1/agents/{agentID}/evolution/suggestions/{suggestionID}` | auth | 更新建议 |

### 5.9 Vault 知识库

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/vault/documents` | auth | 全局文档列表 |
| POST | `/v1/vault/documents` | auth | 创建文档 |
| GET | `/v1/vault/documents/{docID}` | auth | 获取文档 |
| PUT | `/v1/vault/documents/{docID}` | auth | 更新文档 |
| DELETE | `/v1/vault/documents/{docID}` | auth | 删除文档 |
| POST | `/v1/vault/search` | auth | 搜索文档 |
| POST | `/v1/vault/upload` | auth | 上传文档 |
| GET | `/v1/vault/tree` | auth | 目录树 |
| GET | `/v1/vault/graph` | auth | 图谱视图 |
| POST | `/v1/vault/rescan` | auth | 重新扫描 |
| GET | `/v1/vault/enrichment/status` | auth | 富化状态 |
| POST | `/v1/vault/enrichment/stop` | auth | 停止富化 |
| POST | `/v1/vault/links` | auth | 创建链接 |
| POST | `/v1/vault/links/batch` | auth | 批量链接 |
| DELETE | `/v1/vault/links/{linkID}` | auth | 删除链接 |
| GET | `/v1/agents/{agentID}/vault/documents` | auth | Agent 文档 |
| POST | `/v1/agents/{agentID}/vault/documents` | auth | 创建 Agent 文档 |
| GET | `/v1/agents/{agentID}/vault/documents/{docID}` | auth | 获取 Agent 文档 |
| PUT | `/v1/agents/{agentID}/vault/documents/{docID}` | auth | 更新 Agent 文档 |
| DELETE | `/v1/agents/{agentID}/vault/documents/{docID}` | auth | 删除 Agent 文档 |
| GET | `/v1/agents/{agentID}/vault/documents/{docID}/links` | auth | Agent 文档链接 |
| POST | `/v1/agents/{agentID}/vault/links` | auth | 创建 Agent 链接 |
| DELETE | `/v1/agents/{agentID}/vault/links/{linkID}` | auth | 删除 Agent 链接 |
| POST | `/v1/agents/{agentID}/vault/search` | auth | Agent 搜索 |

### 5.10 团队

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/teams/{id}/events` | auth | 团队事件 |
| GET | `/v1/teams/{id}/export/preview` | admin | 导出预览 |
| GET | `/v1/teams/{id}/export` | admin | 导出 |
| POST | `/v1/teams/import` | admin | 导入 |
| GET | `/v1/teams/{teamId}/attachments/{attachmentId}/download` | auth | 下载附件 |
| POST | `/v1/teams/{teamId}/workspace/upload` | auth | 上传工作区文件 |
| PUT | `/v1/teams/{teamId}/workspace/move` | auth | 移动工作区文件 |

### 5.11 追踪 & 使用量

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/traces` | auth | 追踪列表 |
| GET | `/v1/traces/{traceID}` | auth | 追踪详情 |
| GET | `/v1/traces/follow` | auth | 追踪追踪 |
| GET | `/v1/traces/{traceID}/export` | auth | 导出追踪 |
| GET | `/v1/usage/summary` | auth | 使用量摘要 |
| GET | `/v1/usage/timeseries` | auth | 时间序列 |
| GET | `/v1/usage/breakdown` | auth | 明细分类 |
| GET | `/v1/costs/summary` | auth | 成本摘要 |
| GET | `/v1/usage-caps/policies` | auth | 配额策略 |
| POST | `/v1/usage-caps/policies` | auth | 创建策略 |
| PATCH | `/v1/usage-caps/policies/{id}` | auth | 更新策略 |
| DELETE | `/v1/usage-caps/policies/{id}` | auth | 删除策略 |
| GET | `/v1/usage-caps/events` | auth | 配额事件 |
| GET | `/v1/usage-caps/utilization` | auth | 配额使用率 |

### 5.12 Webhook

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/webhooks` | admin | 列出 Webhook |
| POST | `/v1/webhooks` | admin | 创建 Webhook |
| GET | `/v1/webhooks/{id}` | admin | 获取 Webhook |
| PATCH | `/v1/webhooks/{id}` | admin | 更新 Webhook |
| DELETE | `/v1/webhooks/{id}` | admin | 撤销 Webhook |
| POST | `/v1/webhooks/{id}/rotate` | admin | 轮换密钥 |
| POST | `/v1/webhooks/llm` | auth | LLM Webhook 入站 |
| POST | `/v1/webhooks/message` | auth | 消息 Webhook 入站 |

### 5.13 安全 CLI

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/cli-credentials` | auth | 列出凭证 |
| POST | `/v1/cli-credentials` | auth | 创建凭证 |
| GET | `/v1/cli-credentials/{id}` | auth | 获取凭证 |
| PUT | `/v1/cli-credentials/{id}` | auth | 更新凭证 |
| DELETE | `/v1/cli-credentials/{id}` | auth | 删除凭证 |
| POST | `/v1/cli-credentials/{id}/test` | auth | 测试执行 |
| POST | `/v1/cli-credentials/check-binary` | auth | 检查二进制 |
| GET | `/v1/cli-credentials/presets` | auth | 预设列表 |
| GET | `/v1/cli-credentials/{id}/agent-grants` | auth | Agent 授权 |
| POST | `/v1/cli-credentials/{id}/agent-grants` | auth | 创建 Agent 授权 |
| PUT | `/v1/cli-credentials/{id}/agent-grants/{grantId}` | auth | 更新 Agent 授权 |
| DELETE | `/v1/cli-credentials/{id}/agent-grants/{grantId}` | auth | 删除 Agent 授权 |
| POST | `/v1/cli-credentials/{id}/agent-grants/{grantId}/env:reveal` | auth | 揭示环境变量 |
| GET | `/v1/cli-credentials/{id}/user-credentials` | auth | 用户凭证列表 |
| GET | `/v1/cli-credentials/{id}/user-credentials/{userId}` | auth | 获取用户凭证 |
| PUT | `/v1/cli-credentials/{id}/user-credentials/{userId}` | auth | 设置用户凭证 |
| DELETE | `/v1/cli-credentials/{id}/user-credentials/{userId}` | auth | 删除用户凭证 |

### 5.14 工作站

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/workstations` | auth | 列出工作站 |
| POST | `/v1/workstations` | auth | 创建工作站 |
| GET | `/v1/workstations/{id}` | auth | 获取工作站 |
| PUT | `/v1/workstations/{id}` | auth | 更新工作站 |
| DELETE | `/v1/workstations/{id}` | auth | 删除工作站 |
| POST | `/v1/workstations/{id}/test` | auth | 测试连接 |
| GET | `/v1/workstations/{id}/permissions` | auth | 权限列表 |
| POST | `/v1/workstations/{id}/permissions` | auth | 添加权限 |
| PUT | `/v1/workstations/{id}/permissions/{permId}/toggle` | auth | 切换权限 |
| DELETE | `/v1/workstations/{id}/permissions/{permId}` | auth | 删除权限 |
| GET | `/v1/workstations/{id}/activity` | auth | 操作审计 |

### 5.15 备份恢复

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| POST | `/v1/system/backup` | admin | 执行备份 |
| GET | `/v1/system/backup/preflight` | admin | 预检 |
| GET | `/v1/system/backup/download/{token}` | admin | 下载备份 |
| GET | `/v1/system/backup/s3/config` | admin | S3 配置 |
| PUT | `/v1/system/backup/s3/config` | admin | 设置 S3 |
| GET | `/v1/system/backup/s3/list` | admin | S3 备份列表 |
| POST | `/v1/system/backup/s3/upload` | admin | S3 上传 |
| POST | `/v1/system/backup/s3/backup` | admin | S3 备份 |
| POST | `/v1/system/restore` | admin | 系统恢复 |
| POST | `/v1/tenant/backup` | admin | 租户备份 |
| POST | `/v1/tenant/restore` | admin | 租户恢复 |
| GET | `/v1/tenant/backup/preflight` | admin | 租户预检 |
| GET | `/v1/tenant/backup/download/{token}` | admin | 下载租户备份 |

### 5.16 其他

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/v1/edition` | — | 版本信息 |
| GET | `/v1/system-configs` | auth | 系统配置 |
| GET | `/v1/system-configs/{key}` | auth | 获取配置 |
| PUT | `/v1/system-configs/{key}` | admin | 设置配置 |
| DELETE | `/v1/system-configs/{key}` | admin | 删除配置 |
| GET | `/v1/tools/builtin` | auth | 内建工具列表 |
| GET | `/v1/tools/builtin/{name}` | auth | 获取工具 |
| PUT | `/v1/tools/builtin/{name}` | admin | 更新工具 |
| GET | `/v1/tools/builtin/{name}/tenant-config` | admin | 租户配置 |
| PUT | `/v1/tools/builtin/{name}/tenant-config` | admin | 设置租户配置 |
| DELETE | `/v1/tools/builtin/{name}/tenant-config` | admin | 删除租户配置 |
| GET | `/v1/tenants` | admin | 租户列表 |
| POST | `/v1/tenants` | admin | 创建租户 |
| GET | `/v1/tenants/{id}` | admin | 获取租户 |
| PATCH | `/v1/tenants/{id}` | admin | 更新租户 |
| GET | `/v1/tenants/{id}/users` | admin | 租户用户 |
| POST | `/v1/tenants/{id}/users` | admin | 添加用户 |
| DELETE | `/v1/tenants/{id}/users/{userId}` | admin | 移除用户 |
| GET | `/v1/voices` | auth | 语音列表 |
| POST | `/v1/voices/refresh` | admin | 刷新语音 |
| GET | `/v1/tts/capabilities` | auth | TTS 能力 |
| GET | `/v1/tts/config` | auth | TTS 配置 |
| POST | `/v1/tts/config` | admin | 设置 TTS |
| POST | `/v1/tts/synthesize` | auth | 合成语音 |
| POST | `/v1/tts/test-connection` | admin | 测试 TTS |
| GET | `/v1/pending-messages` | auth | 待处理消息分组 |
| GET | `/v1/pending-messages/messages` | auth | 待处理消息详情 |
| POST | `/v1/pending-messages/compact` | auth | 压缩消息 |
| DELETE | `/v1/pending-messages` | auth | 删除消息 |
| GET | `/v1/activity` | auth | 活动日志 |
| GET | `/v1/activity/aggregate` | auth | 活动聚合 |
| GET | `/v1/logs/runtime/aggregate` | admin | 运行日志聚合 |
| GET | `/v1/files/{path...}` | auth | 文件服务 |
| POST | `/v1/files/sign` | — | 签名 URL |
| GET | `/v1/storage/files` | auth | 存储文件列表 |
| GET | `/v1/storage/files/{path...}` | auth | 读取存储文件 |
| POST | `/v1/storage/files` | admin | 上传文件 |
| DELETE | `/v1/storage/files/{path...}` | admin | 删除文件 |
| PUT | `/v1/storage/move` | admin | 移动文件 |
| GET | `/v1/storage/size` | auth | 存储大小 |
| GET | `/v1/packages` | auth | 技能包列表 |
| GET | `/v1/packages/updates` | auth | 可用更新 |
| GET | `/v1/packages/runtimes` | auth | 运行时状态 |
| GET | `/v1/packages/github-releases` | auth | GitHub Release |
| POST | `/v1/packages/install` | admin | 安装包 |
| POST | `/v1/packages/uninstall` | admin | 卸载包 |
| POST | `/v1/packages/update` | admin | 更新包 |
| POST | `/v1/packages/updates/apply-all` | admin | 全部更新 |
| POST | `/v1/packages/updates/refresh` | admin | 刷新更新 |
| GET | `/v1/api-keys` | admin | API Key 列表 |
| POST | `/v1/api-keys` | admin | 创建 API Key |
| POST | `/v1/api-keys/{id}/revoke` | admin | 撤销 API Key |
| POST | `/v1/browser/cookies/sync` | operator | 同步 Cookie |
| GET | `/v1/browser/cookies` | operator | 列出 Cookie |
| DELETE | `/v1/browser/cookies` | operator | 删除 Cookie |
| GET | `/v1/shell-deny-groups` | auth | 拒绝组列表 |
| GET | `/v1/agents/{id}/v3-flags` | auth | V3 标志 |
| PATCH | `/v1/agents/{id}/v3-flags` | auth | 切换 V3 标志 |
| GET | `/v1/agents/{id}/orchestration` | auth | 编排模式 |
| GET | `/v1/system/gateway/upgrade/status` | admin | 升级状态 |
| POST | `/v1/system/gateway/upgrade` | admin | 执行升级 |
| GET | `/v1/openapi.json` | — | OpenAPI 文档 |
| GET | `/v1/docs` | — | Swagger UI |

### 5.17 OAuth

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| POST | `/v1/auth/openai/start` | auth | 开始 OpenAI OAuth |
| POST | `/v1/auth/openai/callback` | auth | OpenAI 回调 |
| POST | `/v1/auth/openai/logout` | auth | 退出 OpenAI |
| GET | `/v1/auth/openai/status` | auth | OpenAI 状态 |
| GET | `/v1/auth/openai/quota` | auth | OpenAI 配额 |
| POST | `/v1/auth/chatgpt/{provider}/start` | auth | 开始 ChatGPT OAuth |
| POST | `/v1/auth/chatgpt/{provider}/callback` | auth | ChatGPT 回调 |
| POST | `/v1/auth/chatgpt/{provider}/logout` | auth | 退出 ChatGPT |
| GET | `/v1/auth/chatgpt/{provider}/status` | auth | ChatGPT 状态 |
| GET | `/v1/auth/chatgpt/{provider}/quota` | auth | ChatGPT 配额 |

### 5.18 OpenAI 兼容 API

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| POST | `/v1/chat/completions` | auth | Chat Completions |
| POST | `/v1/responses` | auth | Responses API |
| POST | `/v1/tools/invoke` | auth | 工具调用 |
| POST | `/mcp/bridge` | auth | MCP Bridge |

### 5.19 WebSocket & 系统

| 方法 | 路由 | 权限 | 用途 |
|------|------|------|------|
| GET | `/ws` | — | WebSocket 连接 |
| GET | `/health` | — | 健康检查 |
| GET | `/v1/media/{id}` | auth | 媒体文件 |
| POST | `/v1/media/upload` | auth | 上传媒体 |
| GET | `/v1/agents/{id}/wake` | — | 唤醒 Agent |

---

## 6. MCP 工具完整列表

### 6.1 文件操作

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `read_file` | `NewReadFileTool` / `NewSandboxedReadFileTool` | 读取文件内容 |
| `write_file` | `NewWriteFileTool` / `NewSandboxedWriteFileTool` | 写入文件内容 |
| `list_files` | `NewListFilesTool` / `NewSandboxedListFilesTool` | 列出目录文件 |
| `edit` | `NewEditTool` / `NewSandboxedEditTool` | 精确文本编辑 |
| `send_file` | `NewSendFileTool` | 发送文件到聊天 |

### 6.2 Shell 执行

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `exec` | `NewExecTool` / `NewSandboxedExecTool` | 执行 Shell 命令 |
| `workstation_exec` | `NewWorkstationExecTool` | 远程工作站执行 |
| `claude_remote` | `NewClaudeRemoteTool` | Claude 远程执行 |
| `git` | (内置) | Git 操作 |
| `psql` | (内置) | PostgreSQL 操作 |

### 6.3 网络

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `web_fetch` | `NewWebFetchTool` | 抓取网页内容 |
| `web_search` | `NewWebSearchTool` | 搜索网页（Brave/DDG/Tavily/Exa） |
| `defuddle` | (内置) | 内容提取 |

### 6.4 记忆 & 知识图谱

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `memory_search` | `NewMemorySearchTool` | 搜索语义记忆 |
| `memory_get` | `NewMemoryGetTool` | 获取记忆块 |
| `memory_expand` | `NewMemoryExpandTool` | 展开记忆上下文 |
| `knowledge_graph_search` | `NewKnowledgeGraphSearchTool` | 搜索知识图谱 |
| `vault_read` | `NewVaultReadTool` | 读取 Vault 文档 |
| `vault_search` | `NewVaultSearchTool` | 搜索 Vault |

### 6.5 媒体

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `read_image` | `NewReadImageTool` | 分析图片 |
| `read_audio` | `NewReadAudioTool` | 分析音频 |
| `read_video` | `NewReadVideoTool` | 分析视频 |
| `read_document` | `NewReadDocumentTool` | 分析文档 |
| `create_image` | `NewCreateImageTool` | 生成图片 |
| `create_video` | `NewCreateVideoTool` | 生成视频 |
| `create_audio` | `NewCreateAudioTool` | 生成音频 |
| `tts` | `NewTtsTool` | 文字转语音 |

### 6.6 Agent 管理

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `delegate` | `NewDelegateTool` | 委派给其他 Agent |
| `spawn` | `NewSpawnTool` | 生成子 Agent |
| `message` | `NewMessageTool` | 发送消息到通道 |
| `sessions_list` | `NewSessionsListTool` | 列出会话 |
| `sessions_send` | `NewSessionsSendTool` | 跨会话发消息 |
| `sessions_history` | `NewSessionsHistoryTool` | 会话历史 |
| `session_status` | `NewSessionStatusTool` | 会话状态 |

### 6.7 团队

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `team_tasks` | `NewTeamTasksTool` | 团队任务管理 |
| `list_group_members` | `NewListGroupMembersTool` | 群组成员列表 |

### 6.8 技能

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `skill_search` | `NewSkillSearchTool` | 搜索技能 |
| `use_skill` | `NewUseSkillTool` | 标记使用技能 |
| `skill_manage` | `NewSkillManageTool` | 管理技能 |
| `publish_skill` | `NewPublishSkillTool` | 发布技能 |

### 6.9 定时 & 心跳

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `cron` | `NewCronTool` | 管理定时任务 |
| `heartbeat` | `NewHeartbeatTool` | 管理心跳 |
| `datetime` | `NewDateTimeTool` | 获取日期时间 |

### 6.10 Telegram

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `create_forum_topic` | `NewCreateForumTopicTool` | 创建论坛话题 |

### 6.11 等待

| 工具名 | 函数 | 用途 |
|--------|------|------|
| `wait` | `NewWaitTool` | 延迟执行 |

---

## 7. 文件索引

| 文件名 | 大小 | 内容 |
|--------|------|------|
| `runGateway_analysis.md` | 31KB | runGateway 启动流程完整分析 |
| `sandbox_analysis.md` | 39KB | 沙箱启动/隔离/状态保存恢复 |
| `prompt_context_rag_analysis.md` | 32KB | 提示词/上下文/RAG/向量化 |
| `skills_analysis.md` | 27KB | 技能注册/管理/发现/权限 |
| `storage_analysis.md` | 33KB | 73 张表完整 Schema |
| `files_functions_index.md` | 30KB | 1299 个 Go 文件函数索引 |
| `api_interfaces_analysis.md` | 本文档 | HTTP/WS/MCP 接口 + 协议 |
| `acp_protocol_analysis.md` | — | ACP 协议详解 |
| `a2a_vs_acp_analysis.md` | 16KB | A2A vs ACP 对比 |
| `protocols_technologies_overview.md` | 27KB | 全协议与技术栈总览 |
