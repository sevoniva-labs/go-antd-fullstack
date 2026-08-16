# RocketMQ 5.x Adapter Slot

Forge 的根模块定义了厂商无关 `messaging.Bus`，内置 Kafka Provider。RocketMQ 5.x 预留配置和 Provider 名称，但**根模块不会在未引入官方 SDK 的情况下宣称已支持**。

接入要求：

1. 优先使用 Apache RocketMQ 5.x 官方 Go client / gRPC 协议。
2. 实现 `messaging.Bus` 的 `Publish/Ping/Close/Provider`。
3. AccessKey/SecretKey 仅从 Secret Provider / 环境变量注入。
4. 生产必须验证：普通/顺序/延迟/事务消息语义、重试和 DLQ、消费幂等、TLS、ACL、多实例、故障切换。
5. 通过 Outbox 发布业务事件，禁止在数据库事务提交前直接发送远端消息。

选择 `messaging.provider=rocketmq` 而未安装适配器时，Forge 会明确启动失败，避免静默降级。
