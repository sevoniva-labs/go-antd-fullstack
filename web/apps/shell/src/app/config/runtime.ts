export type ThemeMode = 'light' | 'dark'
export type LayoutMode = 'mix' | 'side' | 'top'

export interface RuntimeConfig {
  appName: string
  appShortName: string
  description: string
  environment: string
  apiBaseUrl: string
  localLoginEnabled: boolean
  showOrganizationField: boolean
  defaultOrganization: string
  oidcLoginUrl: string
  helpUrl: string
  logoUrl: string
  primaryColor: string
  defaultTheme: ThemeMode
  compactMode: boolean
  layoutMode: LayoutMode
  pageTabs: boolean
  showEnvironmentBadge: boolean
  componentPlayground: boolean
  footerText: string
}

const defaults: RuntimeConfig = {
  appName: 'Sevoniva Forge',
  appShortName: 'Forge',
  description: 'Enterprise Go + React Application Foundation',
  environment: 'DEV',
  apiBaseUrl: '/api/v1',
  localLoginEnabled: true,
  showOrganizationField: false,
  defaultOrganization: 'default',
  oidcLoginUrl: '',
  helpUrl: '',
  logoUrl: '',
  primaryColor: '#1677FF',
  defaultTheme: 'light',
  compactMode: false,
  layoutMode: 'mix',
  pageTabs: true,
  showEnvironmentBadge: true,
  componentPlayground: true,
  footerText: '',
}

export const runtimeConfig: RuntimeConfig = {
  ...defaults,
  ...(window.__FORGE_CONFIG__ ?? {}),
}

export function environmentTone(value = runtimeConfig.environment) {
  const env = value.toUpperCase()
  if (env === 'PROD' || env === 'PRODUCTION') return 'red'
  if (env === 'PRE' || env === 'PREPROD') return 'orange'
  if (env === 'UAT') return 'gold'
  if (env === 'SIT' || env === 'TEST') return 'blue'
  return 'cyan'
}

export function isProductionEnvironment(value = runtimeConfig.environment) {
  const env = value.toUpperCase()
  return env === 'PROD' || env === 'PRODUCTION'
}
