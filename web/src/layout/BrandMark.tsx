import { runtimeConfig } from '../app/config/runtime'

export function BrandMark() {
  if (runtimeConfig.logoUrl) return <img src={runtimeConfig.logoUrl} alt={runtimeConfig.appShortName} className="brand-logo-image" />
  return <div className="brand-mark" aria-label={runtimeConfig.appShortName}>{runtimeConfig.appShortName.slice(0, 1).toUpperCase()}</div>
}
