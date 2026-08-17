import { afterEach, describe, expect, it } from 'vitest'

import { readDocumentCSPNonce } from './csp'

afterEach(() => {
  document.head.innerHTML = ''
})

describe('document CSP nonce', () => {
  it('accepts the 192-bit Base64 nonce injected by the static server', () => {
    document.head.innerHTML = '<meta name="forge-csp-nonce" content="AbCdEfGhIjKlMnOpQrStUvWxYz012345">'
    expect(readDocumentCSPNonce()).toBe('AbCdEfGhIjKlMnOpQrStUvWxYz012345')
  })

  it('ignores an unreplaced marker or malformed value', () => {
    document.head.innerHTML = '<meta name="forge-csp-nonce" content="__FORGE_CSP_NONCE__">'
    expect(readDocumentCSPNonce()).toBeUndefined()
    document.head.innerHTML = '<meta name="forge-csp-nonce" content="attacker-controlled">'
    expect(readDocumentCSPNonce()).toBeUndefined()
  })
})
