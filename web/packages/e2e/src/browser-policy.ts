export const PLAYWRIGHT_DOMESTIC_DOWNLOAD_HOST =
  'https://npmmirror.com/mirrors/playwright'
export const PLAYWRIGHT_VERSION = '1.62.1'
export const PLAYWRIGHT_CHROMIUM_REVISION = '1234'

export function chromiumArtifactName(platform: NodeJS.Platform, architecture: string): string {
  if (platform === 'linux' && architecture === 'arm64') {
    return 'chromium-linux-arm64.zip'
  }
  throw new Error(
    'No validated public domestic Chromium artifact for this platform; use an internal artifact or PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH',
  )
}

export function chromiumDomesticUrl(platform: NodeJS.Platform, architecture: string): string {
  return [
    PLAYWRIGHT_DOMESTIC_DOWNLOAD_HOST,
    'builds/chromium',
    PLAYWRIGHT_CHROMIUM_REVISION,
    chromiumArtifactName(platform, architecture),
  ].join('/')
}
