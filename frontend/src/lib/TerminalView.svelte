<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte'
  import { Terminal } from '@xterm/xterm'
  import { FitAddon } from '@xterm/addon-fit'
  import { WebLinksAddon } from '@xterm/addon-web-links'
  import '@xterm/xterm/css/xterm.css'
  import { Connect, Write, Resize, Disconnect } from '../../wailsjs/go/main/App'
  import { EventsOn, EventsOff, BrowserOpenURL } from '../../wailsjs/runtime/runtime'
  import type { Status } from './types'

  let {
    sessionId,
    hostId,
    active,
    onstatus,
    onlogout,
  }: {
    sessionId: string
    hostId: string
    active: boolean
    onstatus: (s: Status) => void
    onlogout: () => void
  } = $props()

  let el: HTMLDivElement
  let term: Terminal | undefined
  let fit: FitAddon | undefined
  let observer: ResizeObserver | undefined

  // A tab's sessionId is fixed for its lifetime.
  const dataEvent = untrack(() => `ssh:data:${sessionId}`)
  const closedEvent = untrack(() => `ssh:closed:${sessionId}`)

  function decode(b64: string): Uint8Array {
    const bin = atob(b64)
    const out = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i)
    return out
  }

  function refit() {
    if (!term || !fit || !active || el.clientWidth === 0) return
    fit.fit()
  }

  async function connect() {
    if (!term) return
    onstatus('connecting')
    term.write('\x1b[90mMenghubungkan...\x1b[0m\r\n')
    try {
      await Connect(sessionId, hostId, term.cols, term.rows)
      onstatus('connected')
    } catch (e) {
      term.write(`\x1b[31m${e}\x1b[0m\r\n`)
      onstatus('error')
    }
  }

  onMount(() => {
    term = new Terminal({
      scrollback: 5000,
      cursorBlink: true,
      fontFamily: '"Cascadia Mono", Consolas, "Courier New", monospace',
      fontSize: 14,
      theme: { background: '#161616', foreground: '#e8eaeb', cursor: '#00e5fc', selectionBackground: '#00e5fc44' },
    })
    fit = new FitAddon()
    term.loadAddon(fit)
    term.loadAddon(new WebLinksAddon((_e, uri) => BrowserOpenURL(uri)))
    term.open(el)
    refit()

    EventsOn(dataEvent, (b64: string) => term?.write(decode(b64)))
    EventsOn(closedEvent, (loggedOut: boolean) => {
      if (loggedOut) {
        onlogout()
        return
      }
      term?.write('\r\n\x1b[31m[koneksi terputus]\x1b[0m\r\n')
      onstatus('closed')
    })
    term.onData((d) => Write(sessionId, d).catch(() => {}))
    term.onResize(({ cols, rows }) => Resize(sessionId, cols, rows).catch(() => {}))

    observer = new ResizeObserver(refit)
    observer.observe(el)
    connect()
  })

  $effect(() => {
    if (active) {
      requestAnimationFrame(() => {
        refit()
        term?.focus()
      })
    }
  })

  onDestroy(() => {
    EventsOff(dataEvent)
    EventsOff(closedEvent)
    observer?.disconnect()
    Disconnect(sessionId)
    term?.dispose()
  })

  export function reconnect() {
    term?.reset()
    connect()
  }
</script>

<div class="term" bind:this={el}></div>

<style>
  .term {
    position: absolute;
    inset: 0;
    padding: 8px 0 0 10px;
    background: var(--term-bg);
  }
</style>
