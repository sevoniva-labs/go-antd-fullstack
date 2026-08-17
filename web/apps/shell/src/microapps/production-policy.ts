const PRODUCTION_WUJIE_APPROVAL_ERROR = 'PRODUCTION_WUJIE_APPROVAL_REQUIRED'

export interface ProductionWujieApprovalInput {
  production: boolean
  buildTimeApproved: boolean
  runtimes: ReadonlyArray<'wujie' | 'iframe' | undefined>
}

export class ProductionWujieApprovalError extends Error {
  readonly code = PRODUCTION_WUJIE_APPROVAL_ERROR

  constructor() {
    super('production Wujie runtime requires explicit build-time security approval')
    this.name = 'ProductionWujieApprovalError'
  }
}

export function assertProductionWujieApproval(input: ProductionWujieApprovalInput): void {
  const includesWujie = input.runtimes.some((runtime) => runtime === 'wujie')
  if (input.production && includesWujie && !input.buildTimeApproved) {
    throw new ProductionWujieApprovalError()
  }
}

export function isProductionWujieApprovalError(error: unknown): boolean {
  return error instanceof ProductionWujieApprovalError
}
