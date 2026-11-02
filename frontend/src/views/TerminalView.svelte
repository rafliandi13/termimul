<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte'
  import { Terminal } from '@xterm/xterm'
  import { FitAddon } from '@xterm/addon-fit'
  import { WebLinksAddon } from '@xterm/addon-web-links'
  import '@xterm/xterm/css/xterm.css'
  import * as TerminalSvc from '../../wailsjs/go/services/TerminalService'
  import { EventsOn, EventsOff, BrowserOpenURL, ClipboardGetText, ClipboardSetText } from '../../wailsjs/runtime/runtime'
  import type { Status } from '../lib/types'

  let {
    sessionId,
    hostId,
    active,
    focused,
    onstatus,
    onlogout,
    ondata,
    onfocus,
    onmenu,
    ownskey,
  }: {
    sessionId: string
    hostId: string
    active: boolean // the tab is in front
    focused: boolean // this is the tab's focused pane
    onstatus: (s: Status) => void
    onlogout: () => void
    ondata: (data: string) => void
    onfocus: () => void
    onmenu: (x: number, y: number, hasSelection: boolean) => void
    ownskey: (e: KeyboardEvent) => boolean // the tab handles this key; don't send it to the server
  } = $props()

  let el: HTMLDivElement
  let term: Terminal | undefined
  let fit: FitAddon | undefined
  let observer: ResizeObserver | undefined
  let refitTimer: ReturnType<typeof setTimeout> | undefined
  let status = $state<Status>('connecting')

  // A pane's sessionId is fixed for its lifetime.
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

  function copySelection() {
    const text = term?.getSelection()
    if (!text) return
    ClipboardSetText(text)
    term?.clearSelection()
  }

  async function pasteClipboard() {
    const text = await ClipboardGetText()
    // term.paste handles bracketed paste mode and line endings, then fires onData.
    if (text) term?.paste(text)
  }

  // Ctrl+C copies when text is selected and is still SIGINT otherwise; Ctrl+V
  // pastes. Ctrl+Shift+C/V and Cmd+C/V (macOS) do the same. Returning false
  // stops xterm from also sending the key to the server.
  function handleKey(e: KeyboardEvent): boolean {
    if (ownskey(e)) return false
    if (!(e.ctrlKey || e.metaKey) || e.altKey) return true
    const key = e.key.toLowerCase()
    if (key === 'c' && (term?.hasSelection() || e.shiftKey || e.metaKey)) {
      if (e.type === 'keydown') {
        e.preventDefault()
        copySelection()
      }
      return false
    }
    if (key === 'v') {
      if (e.type === 'keydown') {
        e.preventDefault() // also suppresses the browser's own paste event, so text isn't pasted twice
        pasteClipboard()
      }
      return false
    }
    return true
  }

  function onContextMenu(e: MouseEvent) {
    e.preventDefault()
    onmenu(e.clientX, e.clientY, !!term?.hasSelection())
  }

  function setStatus(s: Status) {
    status = s
    onstatus(s)
  }

  async function connect() {
    if (!term) return
    setStatus('connecting')
    term.write('\x1b[90mMenghubungkan...\x1b[0m\r\n')
    try {
      await TerminalSvc.Connect(sessionId, hostId, term.cols, term.rows)
      setStatus('connected')
    } catch (e) {
      term.write(`\x1b[31m${e}\x1b[0m\r\n`)
      setStatus('error')
    }
  }

  onMount(() => {
    term = new Terminal({
      scrollback: 5000,
      cursorBlink: true,
      fontFamily: getComputedStyle(document.documentElement).getPropertyValue('--font-mono').trim() || 'monospace',
      fontSize: 14,
      theme: { background: '#161616', foreground: '#e8eaeb', cursor: '#00e5fc', selectionBackground: '#00e5fc44' },
    })
    fit = new FitAddon()
    term.loadAddon(fit)
    term.loadAddon(new WebLinksAddon((_e, uri) => BrowserOpenURL(uri)))
    term.open(el)
    term.attachCustomKeyEventHandler(handleKey)
    el.addEventListener('contextmenu', onContextMenu)
    el.addEventListener('focusin', onfocus)
    refit()

    EventsOn(dataEvent, (b64: string) => term?.write(decode(b64)))
    EventsOn(closedEvent, (loggedOut: boolean) => {
      if (loggedOut) {
        onlogout()
        return
      }
      term?.write('\r\n\x1b[31m[koneksi terputus]\x1b[0m\r\n')
      setStatus('closed')
    })
    term.onData(ondata)
    term.onResize(({ cols, rows }) => TerminalSvc.Resize(sessionId, cols, rows).catch(() => {}))

    // Coalesced so dragging a divider doesn't send a window-change per pixel.
    observer = new ResizeObserver(() => {
      clearTimeout(refitTimer)
      refitTimer = setTimeout(refit, 30)
    })
    observer.observe(el)
    connect()
  })

  $effect(() => {
    if (active) {
      const focus = focused
      requestAnimationFrame(() => {
        refit()
        if (focus) term?.focus()
      })
    }
  })

  onDestroy(() => {
    EventsOff(dataEvent)
    EventsOff(closedEvent)
    observer?.disconnect()
    clearTimeout(refitTimer)
    el?.removeEventListener('contextmenu', onContextMenu)
    el?.removeEventListener('focusin', onfocus)
    TerminalSvc.Disconnect(sessionId)
    term?.dispose()
  })

  function reconnect() {
    term?.reset()
    connect()
  }

  export function copy() {
    copySelection()
  }

  export function paste() {
    pasteClipboard()
  }

  export function focus() {
    term?.focus()
  }
</script>

<div class="term" bind:this={el}></div>
{#if status === 'closed' || status === 'error'}
  <button class="primary reconnect" onclick={reconnect}>Hubungkan ulang</button>
{/if}

<style>
  .term {
    position: absolute;
    inset: 0;
    padding: 8px 0 0 10px;
    background: var(--term-bg);
  }
  .reconnect {
    position: absolute;
    right: 20px;
    bottom: 20px;
  }
</style>
