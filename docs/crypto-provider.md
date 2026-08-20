# 密钥来源与密码 Provider

脚手架的业务代码只依赖 `crypto.Provider`，不直接依赖 KMS、HSM、密码机或云厂商 SDK。启动配置通过 `security.crypto_key_source` 选择密钥边界：

- `software`：仅适合开发、测试和临时验证，使用 `FORGE_CRYPTO_KEY` 或 `FORGE_CRYPTO_KEY_FILE` 构造软件 Provider。
- `adapter`：生产默认要求。启动器必须通过 `bootstrap.Options.CryptoProviderFactory` 注入机构批准的 KMS/HSM/密码机 Provider；没有注入时 fail-closed。

生产环境不能因为配置了 `standard` 或 `gm` 就视为完成商用密码部署。`gm` 只表示应用层 SM3/SM4 软件能力基线，SM2、国密 TLS、证书链、密钥生命周期、双人双控、备份恢复和密评仍须由目标密码产品和部署环境提供证据。

适配器至少应负责：身份认证、密钥标识和版本、加解密或信封密钥操作、轮换、双人复核、审计、超时、失败关闭和密钥材料不出设备。适配器必须在机构批准的依赖和内网源中构建，不得把厂商密钥、设备 PIN 或长期秘密写入配置、日志或仓库。

示例启动注入形态：

```go
app, err := bootstrap.New(ctx, bootstrap.Options{
    Version: version,
    CryptoProviderFactory: func(ctx context.Context, security config.Security) (crypto.Provider, error) {
        return approvedHSMProvider(ctx, security)
    },
})
```

当前脚手架提供的是稳定的 `Adapter slot` 和生产 fail-closed 约束，不声称任何具体 KMS/HSM/密码机型号已通过银行、信创或密评认证。目标型号、版本、算法、证书、硬件架构和测试报告完成后，才可将对应记录升级为 `Target-tested`。
