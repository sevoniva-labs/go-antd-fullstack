import type { FormInstance } from 'antd'
import { ApiError } from '../../api/client'

export function applyApiFieldErrors(form: FormInstance, error: unknown): boolean {
  if (!(error instanceof ApiError) || !error.fieldErrors?.length) return false
  form.setFields(error.fieldErrors.map((item) => ({
    name: item.path.split('.').filter(Boolean),
    errors: [item.message || item.code],
  })))
  return true
}
