import { spawn } from 'node:child_process'
import { resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const repositoryRoot = resolve(fileURLToPath(new URL('../../../..', import.meta.url)))
const processes = [
  spawn('pnpm', [
    '--filter', '@forge/shell', 'preview', '--host', '127.0.0.1',
    '--port', '4173', '--strictPort',
  ], { cwd: repositoryRoot, env: process.env, stdio: 'inherit' }),
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
