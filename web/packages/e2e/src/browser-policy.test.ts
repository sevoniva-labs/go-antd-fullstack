import { describe, expect, it } from 'vitest'

import {
  PLAYWRIGHT_VERSION,
  chromiumArtifactName,
  chromiumDomesticUrl,
} from './browser-policy'

describe('Playwright domestic browser policy', () => {
  it('pins the tested Playwright package and Linux ARM64 artifact', () => {
    expect(PLAYWRIGHT_VERSION).toBe('1.62.1')
    expect(chromiumArtifactName('linux', 'arm64')).toBe('chromium-linux-arm64.zip')
    expect(chromiumDomesticUrl('linux', 'arm64')).toBe(
      'https://npmmirror.com/mirrors/playwright/builds/chromium/1234/chromium-linux-arm64.zip',
    )
  })

  it('refuses unverified platform fallbacks', () => {
    expect(() => chromiumArtifactName('darwin', 'arm64')).toThrow(/No validated public domestic/)
    expect(() => chromiumArtifactName('linux', 'x64')).toThrow(/No validated public domestic/)
  })
})
