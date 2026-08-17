import { describe, expect, it } from 'vitest'

import {
  assertProductionWujieApproval,
  isProductionWujieApprovalError,
} from './production-policy'

describe('production Wujie approval policy', () => {
  it('fails closed when any production release uses Wujie without build-time approval', () => {
    let failure: unknown
    try {
      assertProductionWujieApproval({
        production: true,
        buildTimeApproved: false,
        runtimes: ['iframe', 'wujie'],
      })
    } catch (error) {
      failure = error
    }

    expect(isProductionWujieApprovalError(failure)).toBe(true)
  })

  it('accepts an explicitly approved production Wujie release', () => {
    expect(() => assertProductionWujieApproval({
      production: true,
      buildTimeApproved: true,
      runtimes: ['wujie'],
    })).not.toThrow()
  })

  it('does not require a Wujie exception for iframe-only or nonproduction releases', () => {
    expect(() => assertProductionWujieApproval({
      production: true,
      buildTimeApproved: false,
      runtimes: ['iframe', undefined],
    })).not.toThrow()
    expect(() => assertProductionWujieApproval({
      production: false,
      buildTimeApproved: false,
      runtimes: ['wujie'],
    })).not.toThrow()
  })
})
