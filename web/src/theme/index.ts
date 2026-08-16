import type { ThemeConfig } from 'antd'
import { runtimeConfig, type ThemeMode } from '../app/config/runtime'

export function createForgeTheme(mode: ThemeMode): ThemeConfig {
  const dark = mode === 'dark'
  return {
    token: {
      colorPrimary: runtimeConfig.primaryColor,
      colorInfo: runtimeConfig.primaryColor,
      colorSuccess: '#16A34A',
      colorWarning: '#D97706',
      colorError: '#DC2626',
      borderRadius: 8,
      borderRadiusLG: 12,
      controlHeight: 36,
      fontSize: 14,
      colorBgLayout: dark ? '#0B1220' : '#F5F7FA',
    },
    components: {
      Layout: {
        bodyBg: dark ? '#0B1220' : '#F5F7FA',
        headerBg: dark ? '#111827' : '#FFFFFF',
        siderBg: dark ? '#0F172A' : '#FFFFFF',
      },
      Menu: {
        itemBorderRadius: 8,
        itemMarginInline: 8,
      },
      Table: {
        headerBg: dark ? '#111827' : '#FAFAFA',
        headerBorderRadius: 8,
      },
      Card: {
        borderRadiusLG: 12,
      },
      Button: {
        borderRadius: 8,
        controlHeight: 36,
      },
      Input: {
        borderRadius: 8,
        controlHeight: 38,
      },
      Select: {
        borderRadius: 8,
        controlHeight: 38,
      },
      Modal: {
        borderRadiusLG: 12,
      },
      Drawer: {
        borderRadiusLG: 12,
      },
      Tabs: {
        cardBg: dark ? '#111827' : '#FFFFFF',
      },
    },
  }
}
