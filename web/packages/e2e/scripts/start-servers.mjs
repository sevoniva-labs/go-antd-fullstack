import { spawn } from 'node:child_process'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const repositoryRoot = resolve(fileURLToPath(new URL('../../../..', import.meta.url)))
const processes = [
  spawn('go', ['run', './web/packages/e2e/scripts/csp-server.go'], {
    cwd: repositoryRoot,
    env: {
      ...process.env,
      FORGE_E2E_STATIC_ADDR: '127.0.0.1:4173',
      FORGE_E2E_FRAME_SOURCES: 'http://127.0.0.1:4181',
      FORGE_E2E_CONNECT_SOURCES: 'http://127.0.0.1:4181',
      FORGE_E2E_WUJIE_CSP: 'true',
      GOPROXY: 'https://goproxy.cn',
      GOSUMDB: 'sum.golang.org https://goproxy.cn/sumdb/sum.golang.org',
    },
    stdio: 'inherit',
  }),
  spawn('go', ['run', './web/packages/e2e/scripts/csp-server.go'], {
    cwd: repositoryRoot,
    env: {
      ...process.env,
      FORGE_E2E_STATIC_ADDR: '127.0.0.1:4190',
      GOPROXY: 'https://goproxy.cn',
      GOSUMDB: 'sum.golang.org https://goproxy.cn/sumdb/sum.golang.org',
    },
    stdio: 'inherit',
  }),
  spawn('pnpm', [
    '--filter', '@forge/example-remote', 'preview', '--host', '127.0.0.1',
  ], { cwd: repositoryRoot, env: process.env, stdio: 'inherit' }),
]

let stopping = false
function stop(signal = 'SIGTERM') {
  if (stopping) return
  stopping = true
  processes.forEach((child) => child.kill(signal))
}

process.on('SIGINT', () => stop('SIGINT'))
process.on('SIGTERM', () => stop('SIGTERM'))
processes.forEach((child) => {
  child.on('exit', (code) => {
    if (!stopping) {
      stop()
      process.exitCode = code ?? 1
    }
  })
})

await new Promise(() => {})
