import type { vault } from '../../wailsjs/go/models'
import * as Snippets from '../../wailsjs/go/services/SnippetService'
import { EventsOn } from '../../wailsjs/runtime/runtime'

// Payload of "snippet:run" events (RunEvent in services/snippets.go).
interface RunEvent {
  runId: string
  hostId: string
  kind: 'start' | 'output' | 'done' | 'finished'
  stream?: 'stdout' | 'stderr'
  data?: string
  exitCode: number
  error?: string
  durationMs: number
}

export interface Chunk {
  stream: 'stdout' | 'stderr'
  text: string
}

export interface HostResult {
  hostId: string
  label: string
  state: 'queued' | 'running' | 'done'
  chunks: Chunk[]
  exitCode: number
  error: string
  durationMs: number
}

// The current batch lives here rather than in SnippetsView, so its output keeps
// streaming while the user looks at another screen.
class SnippetRun {
  id = $state<string | null>(null)
  command = $state('')
  running = $state(false)
  results = $state<HostResult[]>([])

  #pending = false // Run() hasn't returned the ID yet, but events may already arrive
  #listening = false

  async start(command: string, hosts: vault.Host[], concurrency: number, timeoutSec: number) {
    this.#listen()
    this.command = command
    this.results = hosts.map((h) => ({
      hostId: h.id,
      label: h.label,
      state: 'queued',
      chunks: [],
      exitCode: -1,
      error: '',
      durationMs: 0,
    }))
    this.id = null
    this.running = true
    this.#pending = true
    try {
      const id = await Snippets.Run({ command, hostIds: hosts.map((h) => h.id), concurrency, timeoutSec })
      if (this.#pending) this.id = id
    } catch (err) {
      this.running = false
      throw err
    } finally {
      this.#pending = false
    }
  }

  cancel() {
    if (this.id) Snippets.Cancel(this.id)
  }

  #listen() {
    if (this.#listening) return
    this.#listening = true
    EventsOn('snippet:run', (e: RunEvent) => this.#apply(e))
  }

  #apply(e: RunEvent) {
    if (this.#pending && this.id === null) this.id = e.runId
    if (e.runId !== this.id) return
    if (e.kind === 'finished') {
      this.running = false
      return
    }
    const r = this.results.find((r) => r.hostId === e.hostId)
    if (!r) return
    switch (e.kind) {
      case 'start':
        r.state = 'running'
        break
      case 'output': {
        const last = r.chunks[r.chunks.length - 1]
        if (last?.stream === e.stream) last.text += e.data ?? ''
        else r.chunks.push({ stream: e.stream ?? 'stdout', text: e.data ?? '' })
        break
      }
      case 'done':
        r.state = 'done'
        r.exitCode = e.exitCode
        r.error = e.error ?? ''
        r.durationMs = e.durationMs
        break
    }
  }
}

export const snippetRun = new SnippetRun()
