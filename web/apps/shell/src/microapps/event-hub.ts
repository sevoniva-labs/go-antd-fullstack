import type { HostEventHub } from '@forge/host-sdk'

class ShellEventHub implements HostEventHub {
  readonly #listeners = new Map<string, Set<(payload: unknown) => void>>()

  publish(topic: string, payload: unknown): void {
    for (const listener of this.#listeners.get(topic) ?? []) {
      listener(payload)
    }
  }

  subscribe(topic: string, listener: (payload: unknown) => void): () => void {
    const listeners = this.#listeners.get(topic) ?? new Set()
    listeners.add(listener)
    this.#listeners.set(topic, listeners)
    return () => {
      listeners.delete(listener)
      if (listeners.size === 0) this.#listeners.delete(topic)
    }
  }
}

export const shellEventHub: HostEventHub = new ShellEventHub()
