import { useEffect, useState } from 'react';
import {
  ApiOutlined,
  CheckCircleOutlined,
  DisconnectOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons';
import { Alert, Button, ConfigProvider, Descriptions, Result, Space, Spin, Tag, Typography } from 'antd';

import type { HostSdk } from '@forge/host-sdk';

import { loadRemoteViewModel, resolveHostSdk, type RemoteViewModel } from './host';
import './styles.css';

type RemoteState =
  | Readonly<{ status: 'loading' }>
  | Readonly<{ status: 'standalone' }>
  | Readonly<{ status: 'ready'; hostSdk: HostSdk; viewModel: RemoteViewModel }>
  | Readonly<{ status: 'error'; message: string }>;

export function App() {
  const [state, setState] = useState<RemoteState>({ status: 'loading' });

  useEffect(() => {
    let active = true;
    const hostSdk = resolveHostSdk(window);
    if (!hostSdk) {
      setState({ status: 'standalone' });
      return () => {
        active = false;
      };
    }

    void loadRemoteViewModel(hostSdk)
      .then((viewModel) => {
        if (!active) return;
        hostSdk.report('microapp.ready', { path: window.location.pathname });
        setState({ status: 'ready', hostSdk, viewModel });
      })
      .catch((error: unknown) => {
        if (!active) return;
        setState({
          status: 'error',
          message: error instanceof Error ? error.message : '宿主上下文加载失败',
        });
      });

    return () => {
      active = false;
    };
  }, []);

  if (state.status === 'loading') {
    return (
      <main className="remote-loading">
        <Spin size="large" tip="正在建立受控宿主通道" />
      </main>
    );
  }

  if (state.status === 'standalone') {
    return (
      <main className="remote-standalone">
        <Result
          icon={<DisconnectOutlined />}
          status="info"
          title="远程应用未连接宿主"
          subTitle="独立运行模式不会创建模拟用户、权限或访问凭证。请从 Forge Shell 的受控微应用入口打开。"
        />
      </main>
    );
  }

  if (state.status === 'error') {
    return (
      <main className="remote-standalone">
        <Result status="error" title="宿主校验失败" subTitle={state.message} />
      </main>
    );
  }

  const { hostSdk, viewModel } = state;
  return (
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: viewModel.context.theme.primaryColor,
          borderRadius: 6,
          fontFamily: '"Noto Sans SC", "Source Han Sans SC", sans-serif',
        },
      }}
    >
      <main className="remote-canvas">
        <section className="remote-hero">
          <div>
            <Typography.Text className="remote-eyebrow">GOVERNED MICRO APP</Typography.Text>
            <Typography.Title level={2}>风险工作台示例</Typography.Title>
            <Typography.Paragraph>
              一个独立构建、独立发布，并由 Shell 注入最小权限能力的无界子应用。
            </Typography.Paragraph>
          </div>
          <Tag icon={<CheckCircleOutlined />} color="success">
            Host SDK 已连接
          </Tag>
        </section>

        <Alert
          showIcon
          type="info"
          message="凭证隔离生效"
          description="子应用只拿到经过裁剪的用户上下文和授权能力，不接触 Cookie、Token 或任意请求头。"
        />

        <section className="remote-grid">
          <article className="remote-panel">
            <Space align="center">
              <SafetyCertificateOutlined />
              <Typography.Title level={4}>授权上下文</Typography.Title>
            </Space>
            <Descriptions column={1} size="small">
              <Descriptions.Item label="应用">{hostSdk.appId}@{hostSdk.appVersion}</Descriptions.Item>
              <Descriptions.Item label="当前用户">{viewModel.session.displayName}</Descriptions.Item>
              <Descriptions.Item label="组织">{viewModel.session.organizationId}</Descriptions.Item>
              <Descriptions.Item label="用户读取">
                {viewModel.canReadUsers ? '已授权' : '未授权'}
              </Descriptions.Item>
              <Descriptions.Item label="数据范围">
                {viewModel.session.dataScopes.join(', ') || '无'}
              </Descriptions.Item>
            </Descriptions>
          </article>

          <article className="remote-panel remote-panel-accent">
            <Space align="center">
              <ApiOutlined />
              <Typography.Title level={4}>宿主能力</Typography.Title>
            </Space>
            <Typography.Paragraph>
              API 命名空间、导航路径、事件主题与遥测字段均由应用清单授权并由 Host SDK 运行时强制执行。
            </Typography.Paragraph>
            <Button
              type="primary"
              onClick={() => void hostSdk.navigate('/apps/example-remote')}
            >
              打开受控工作区
            </Button>
          </article>
        </section>
      </main>
    </ConfigProvider>
  );
}
