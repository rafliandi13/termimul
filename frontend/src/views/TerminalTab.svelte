<script lang="ts">
  import { untrack } from 'svelte'
  import type { vault } from '../../wailsjs/go/models'
  import * as TerminalSvc from '../../wailsjs/go/services/TerminalService'
  import PanePicker from '../components/PanePicker.svelte'
  import {
    clampRatio,
    computeLayout,
    isTerminalReport,
    leaves,
    neighbour,
    paneAction,
    removePane,
    setRatio,
    splitPane,
    type Divider,
    type Move,
    type PaneNode,
    type SplitDir,
  } from '../lib/panes'
  import type { Status } from '../lib/types'
  import TerminalView from './TerminalView.svelte'

  let {
    tabId,
    hostId,
    hosts,
    active,
    onstatus,
    ontitle,
    onclose,
  }: {
    tabId: string
    hostId: string
    hosts: vault.Host[]
    active: boolean
    onstatus: (s: Status) => void
    ontitle: (title: string) => void
    onclose: () => void
  } = $props()

  let root = $state<PaneNode>(untrack(() => ({ kind: 'leaf', id: tabId, hostId })))
  let focusedId = $state(untrack(() => tabId))
  let statuses = $state<Record<string, Status>>({})
  let broadcast = $state(false)
  let menu = $state<{ x: number; y: number; hasSelection: boolean } | null>(null)
  let picking = $state(false)
  let box: HTMLDivElement
  let views: Record<string, TerminalView> = {}

  const layout = $derived(computeLayout(root))
  const paneCount = $derived(layout.panes.length)
  const broadcasting = $derived(broadcast && paneCount > 1)
  // Sorted by ID, so a split inserts one element and never moves a live terminal in the DOM.
  const ordered = $derived([...layout.panes].sort((a, b) => (a.pane.id < b.pane.id ? -1 : 1)))
  const focusedHost = $derived(layout.panes.find((p) => p.pane.id === focusedId)?.pane.hostId)

  $effect(() => onstatus(statuses[focusedId] ?? 'connecting'))
  $effect(() => {
    const label = hosts.find((h) => h.id === focusedHost)?.label
    if (label) ontitle(label)
  })

  function split(dir: SplitDir, host = focusedHost) {
    if (!host) return
    const id = crypto.randomUUID()
    root = splitPane(root, focusedId, dir, { kind: 'leaf', id, hostId: host }, crypto.randomUUID())
    focusedId = id
  }

  function closePane(id: string) {
    if (paneCount === 1) return onclose()
    // Stay close to where the user was: the pane that takes over the space.
    const next = (['right', 'down', 'left', 'up'] as Move[]).map((m) => neighbour(layout, id, m)).find(Boolean)
    root = removePane(root, id)
    delete statuses[id]
    delete views[id]
    if (focusedId === id) focusedId = next ?? leaves(root)[0].id
  }

  function send(from: string, data: string) {
    const targets = broadcasting && !isTerminalReport(data) ? leaves(root).map((p) => p.id) : [from]
    for (const id of targets) TerminalSvc.Write(id, data).catch(() => {})
  }

  function act(action: NonNullable<ReturnType<typeof paneAction>>) {
    switch (action) {
      case 'split-right':
        return split('row')
      case 'split-down':
        return split('column')
      case 'close':
        return closePane(focusedId)
      default:
        focusedId = neighbour(layout, focusedId, action) ?? focusedId
    }
  }

  function ownsKey(e: KeyboardEvent) {
    const action = paneAction(e)
    if (!action) return false
    // Alt+arrows belong to the shell when there is no other pane to move to.
    return paneCount > 1 || action === 'split-right' || action === 'split-down' || action === 'close'
  }

  // Keys reach the window after TerminalView has kept them from the server.
  function onkeydown(e: KeyboardEvent) {
    if (!active || picking || !ownsKey(e)) return
    e.preventDefault()
    menu = null
    act(paneAction(e)!)
  }

  function closeMenu() {
    menu = null
    views[focusedId]?.focus()
  }

  function run(action: () => void) {
    menu = null
    action()
    views[focusedId]?.focus()
  }

  function dragDivider(e: PointerEvent, d: Divider) {
    e.preventDefault()
    const handle = e.currentTarget as HTMLElement
    handle.setPointerCapture(e.pointerId)
    const move = (ev: PointerEvent) => {
      const r = box.getBoundingClientRect()
      const ratio =
        d.dir === 'row'
          ? ((ev.clientX - r.left) / r.width - d.area.x) / d.area.w
          : ((ev.clientY - r.top) / r.height - d.area.y) / d.area.h
      root = setRatio(root, d.id, ratio)
    }
    const up = () => {
      handle.removeEventListener('pointermove', move)
      handle.removeEventListener('pointerup', up)
      views[focusedId]?.focus()
    }
    handle.addEventListener('pointermove', move)
    handle.addEventListener('pointerup', up)
  }

  function nudgeDivider(e: KeyboardEvent, d: Divider) {
    const step = { ArrowLeft: -0.02, ArrowUp: -0.02, ArrowRight: 0.02, ArrowDown: 0.02 }[e.key]
    if (!step) return
    e.preventDefault()
    root = setRatio(root, d.id, clampRatio(d.ratio + step))
  }

  const pct = (n: number) => `${n * 100}%`
  const menuX = $derived(menu ? Math.min(menu.x, window.innerWidth - 250) : 0)
  const menuY = $derived(menu ? Math.min(menu.y, window.innerHeight - 320) : 0)
</script>

<svelte:window {onkeydown} />

<div class="panes" bind:this={box}>
  {#each ordered as { pane, rect } (pane.id)}
    <div
      class="slot"
      class:focused={paneCount > 1 && pane.id === focusedId}
      class:broadcasting
      style:left={pct(rect.x)}
      style:top={pct(rect.y)}
      style:width={pct(rect.w)}
      style:height={pct(rect.h)}
    >
      <TerminalView
        bind:this={views[pane.id]}
        sessionId={pane.id}
        hostId={pane.hostId}
        {active}
        focused={pane.id === focusedId}
        onstatus={(s) => (statuses[pane.id] = s)}
        onlogout={() => closePane(pane.id)}
        ondata={(d) => send(pane.id, d)}
        onfocus={() => (focusedId = pane.id)}
        onmenu={(x, y, hasSelection) => {
          focusedId = pane.id
          menu = { x, y, hasSelection }
        }}
        ownskey={ownsKey}
      />
    </div>
  {/each}

  {#each layout.dividers as d (d.id)}
    <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
    <div
      class="divider {d.dir}"
      role="separator"
      tabindex="0"
      aria-label="Ubah ukuran pane"
      aria-orientation={d.dir === 'row' ? 'vertical' : 'horizontal'}
      aria-valuenow={Math.round(d.ratio * 100)}
      style:left={pct(d.dir === 'row' ? d.area.x + d.area.w * d.ratio : d.area.x)}
      style:top={pct(d.dir === 'row' ? d.area.y : d.area.y + d.area.h * d.ratio)}
      style:width={d.dir === 'row' ? null : pct(d.area.w)}
      style:height={d.dir === 'row' ? pct(d.area.h) : null}
      onpointerdown={(e) => dragDivider(e, d)}
      onkeydown={(e) => nudgeDivider(e, d)}
    ></div>
  {/each}

  {#if broadcasting}
    <button class="badge" onclick={() => (broadcast = false)} title="Klik untuk mematikan">
      Input dikirim ke {paneCount} pane
    </button>
  {/if}
</div>

{#if menu}
  <button class="scrim" aria-label="Tutup menu" onclick={closeMenu} oncontextmenu={(e) => (e.preventDefault(), closeMenu())}></button>
  <div class="menu" role="menu" style:left="{menuX}px" style:top="{menuY}px">
    <button role="menuitem" disabled={!menu.hasSelection} onclick={() => run(() => views[focusedId]?.copy())}>Salin</button>
    <button role="menuitem" onclick={() => run(() => views[focusedId]?.paste())}>Tempel</button>
    <hr />
    <button role="menuitem" onclick={() => run(() => split('row'))}>Bagi ke kanan <kbd>Ctrl+Shift+D</kbd></button>
    <button role="menuitem" onclick={() => run(() => split('column'))}>Bagi ke bawah <kbd>Ctrl+Shift+E</kbd></button>
    <button role="menuitem" onclick={() => ((menu = null), (picking = true))}>Bagi dengan host lain…</button>
    <hr />
    <button role="menuitem" disabled={paneCount < 2} onclick={() => run(() => (broadcast = !broadcast))}>
      {broadcast ? 'Matikan' : 'Kirim'} input ke semua pane
    </button>
    <button role="menuitem" onclick={() => run(() => closePane(focusedId))}>Tutup pane <kbd>Ctrl+W</kbd></button>
  </div>
{/if}

{#if picking}
  <PanePicker
    {hosts}
    onpick={(host, dir) => {
      picking = false
      run(() => split(dir, host.id))
    }}
    oncancel={() => {
      picking = false
      views[focusedId]?.focus()
    }}
  />
{/if}

<style>
  .panes {
    position: absolute;
    inset: 0;
    overflow: hidden;
    background: var(--term-bg);
  }
  .slot {
    position: absolute;
    overflow: hidden;
  }
  .slot::after {
    content: '';
    position: absolute;
    inset: 0;
    pointer-events: none;
    border: 1px solid transparent;
  }
  .slot.focused::after {
    border-color: var(--accent);
  }
  .slot.broadcasting::after {
    border-color: #f2c14e;
  }
  .divider {
    position: absolute;
    z-index: 2;
    background: var(--border);
    background-clip: content-box;
    touch-action: none;
  }
  .divider.row {
    width: 7px;
    margin-left: -3px;
    padding: 0 3px;
    cursor: col-resize;
  }
  .divider.column {
    height: 7px;
    margin-top: -3px;
    padding: 3px 0;
    cursor: row-resize;
  }
  .divider:hover,
  .divider:focus-visible {
    background-color: var(--accent);
    outline: none;
  }
  .badge {
    position: absolute;
    top: 8px;
    right: 12px;
    z-index: 3;
    padding: 3px 10px;
    font-size: 12px;
    border-color: #f2c14e;
    color: #f2c14e;
    background: var(--bg-raised);
  }
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 9;
    border: none;
    border-radius: 0;
    background: transparent;
    cursor: default;
  }
  .menu {
    position: fixed;
    z-index: 10;
    width: 240px;
    padding: 4px;
    display: flex;
    flex-direction: column;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: 6px;
    box-shadow: 0 6px 20px #0008;
  }
  .menu button {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    border: none;
    background: transparent;
    padding: 6px 10px;
    font-size: 13px;
    text-align: left;
  }
  .menu button:hover:not(:disabled) {
    background: var(--bg-hover);
  }
  .menu hr {
    width: 100%;
    margin: 4px 0;
    border: none;
    border-top: 1px solid var(--border);
  }
  kbd {
    font-family: inherit;
    font-size: 11px;
    color: var(--text-muted);
  }
</style>
