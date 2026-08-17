const CSP_NONCE_PATTERN = /^[A-Za-z0-9+/]{32}$/

export function readDocumentCSPNonce(documentRoot: Document = document): string | undefined {
  const nonce = documentRoot
    .querySelector<HTMLMetaElement>('meta[name="forge-csp-nonce"]')
    ?.content.trim()
  return nonce && CSP_NONCE_PATTERN.test(nonce) ? nonce : undefined
}
