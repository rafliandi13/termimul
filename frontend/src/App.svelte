<script lang="ts">
  import { onMount } from 'svelte'
  import type { main } from '../wailsjs/go/models'
  import { ListHosts, DeleteHost } from '../wailsjs/go/main/App'
  import HostForm from './lib/HostForm.svelte'
  import TerminalView from './lib/TerminalView.svelte'
  import type { Status, Tab } from './lib/types'

  let hosts = $state<main.Host[]>([])
  let tabs = $state<Tab[]>([])
  let activeTab = $state<string | null>(null) // null = hosts screen
  let editing = $state<main.Host | null | undefined>(undefined) // undefined = form closed, null = new host
  let query = $state('')
  let groupFilter = $state<string | null>(null)
  let views: Record<string, TerminalView> = {}

  const groups = $derived([...new Set(hosts.map((h) => h.group).filter(Boolean))].sort())
  const visible = $derived(
    hosts.filter((h) => {
      if (groupFilter !== null && h.group !== groupFilter) return false
      const q = query.trim().toLowerCase()
      return !q || [h.label, h.address, h.username, h.group].some((v) => v.toLowerCase().includes(q))
    }),
  )
  const grouped = $derived.by(() => {
    const map = new Map<string, main.Host[]>()
    for (const h of visible) {
      const key = h.group || 'Tanpa grup'
      map.set(key, [...(map.get(key) ?? []), h])
    }
    return [...map]
  })

  async function load() {
    hosts = await ListHosts()
  }

  function open(h: main.Host) {
    const id = crypto.randomUUID()
    tabs.push({ id, hostId: h.id, title: h.label, status: 'connecting' })
    activeTab = id
  }

  function close(id: string) {
    const i = tabs.findIndex((t) => t.id === id)
    if (i < 0) return
    tabs.splice(i, 1)
    delete views[id]
    if (activeTab === id) activeTab = tabs[Math.min(i, tabs.length - 1)]?.id ?? null
  }

  function setStatus(id: string, s: Status) {
    const t = tabs.find((t) => t.id === id)
    if (t) t.status = s
  }

  async function remove(h: main.Host) {
    if (!confirm(`Hapus host "${h.label}"?`)) return
    await DeleteHost(h.id)
    await load()
  }

  async function saved() {
    editing = undefined
    await load()
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.ctrlKey && e.key.toLowerCase() === 'w' && activeTab) {
      e.preventDefault()
      close(activeTab)
    } else if (e.ctrlKey && e.key === 'Tab' && tabs.length) {
      e.preventDefault()
      const order = [null, ...tabs.map((t) => t.id)]
      const next = (order.indexOf(activeTab) + (e.shiftKey ? -1 : 1) + order.length) % order.length
      activeTab = order[next]
    }
  }

  onMount(load)
</script>

<svelte:window {onkeydown} />

<div class="shell">
  <nav class="tabs" aria-label="Tab">
    <button class="tab home" class:active={activeTab === null} onclick={() => (activeTab = null)}>
      <svg class="logo" viewBox="540 450 850 690" aria-hidden="true">
        <path d="M584 489H1193V614H952V1098H826V614H584Z" />
        <path d="M1068 890 1168 995 1068 1100" />
        <rect x="1208" y="1070" width="170" height="57" rx="10" />
      </svg>
      Hosts
    </button>
    {#each tabs as t (t.id)}
      <div class="tab" class:active={activeTab === t.id}>
        <button class="tab-label" onclick={() => (activeTab = t.id)} title={t.title}>
          <span class="dot {t.status}" aria-label={t.status}></span>
          {t.title}
        </button>
        <button class="tab-close" aria-label="Tutup tab {t.title}" onclick={() => close(t.id)}>×</button>
      </div>
    {/each}
  </nav>

  <main>
    {#if activeTab === null}
      <div class="hosts">
        <aside>
          <button class="filter" class:on={groupFilter === null} onclick={() => (groupFilter = null)}>
            Semua host <span>{hosts.length}</span>
          </button>
          {#each groups as g}
            <button class="filter" class:on={groupFilter === g} onclick={() => (groupFilter = g)}>
              {g} <span>{hosts.filter((h) => h.group === g).length}</span>
            </button>
          {/each}
        </aside>

        <section>
          <header>
            <input type="search" placeholder="Cari host, alamat, user…" bind:value={query} />
            <button class="primary" onclick={() => (editing = null)}>+ Host baru</button>
          </header>

          {#if hosts.length === 0}
            <div class="empty">
              <p>Belum ada host tersimpan.</p>
              <button class="primary" onclick={() => (editing = null)}>Tambah host pertama</button>
            </div>
          {:else if visible.length === 0}
            <div class="empty"><p>Tidak ada host yang cocok dengan "{query}".</p></div>
          {/if}

          {#each grouped as [group, list] (group)}
            <h3>{group}</h3>
            <div class="grid">
              {#each list as h (h.id)}
                <div class="card">
                  <button class="card-main" onclick={() => open(h)} title="Hubungkan ke {h.label}">
                    <span class="avatar" aria-hidden="true">{h.label.slice(0, 1).toUpperCase()}</span>
                    <span class="meta">
                      <strong>{h.label}</strong>
                      <small>{h.username}@{h.address}{h.port !== 22 ? `:${h.port}` : ''}</small>
                    </span>
                  </button>
                  <div class="card-actions">
                    <button class="ghost" onclick={() => (editing = h)}>Edit</button>
                    <button class="ghost danger" onclick={() => remove(h)}>Hapus</button>
                  </div>
                </div>
              {/each}
            </div>
          {/each}
        </section>
      </div>
    {/if}

    {#each tabs as t (t.id)}
      <div class="pane" class:hidden={activeTab !== t.id}>
        <TerminalView
          bind:this={views[t.id]}
          sessionId={t.id}
          hostId={t.hostId}
          active={activeTab === t.id}
          onstatus={(s) => setStatus(t.id, s)}
          onlogout={() => close(t.id)}
        />
        {#if t.status === 'closed' || t.status === 'error'}
          <button class="primary reconnect" onclick={() => views[t.id]?.reconnect()}>Hubungkan ulang</button>
        {/if}
      </div>
    {/each}
  </main>
</div>

{#if editing !== undefined}
  <HostForm host={editing} {groups} onsaved={saved} oncancel={() => (editing = undefined)} />
{/if}

<style>
  .shell {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .tabs {
    display: flex;
    gap: 2px;
    padding: 6px 6px 0;
    background: var(--bg);
    border-bottom: 1px solid var(--border);
    overflow-x: auto;
    flex-shrink: 0;
  }
  .tab {
    display: flex;
    align-items: center;
    border: 1px solid transparent;
    border-bottom: none;
    border-radius: 6px 6px 0 0;
    background: transparent;
    color: var(--text-muted);
    max-width: 220px;
    flex-shrink: 0;
  }
  .tab.active {
    background: var(--bg-raised);
    border-color: var(--border);
    color: var(--text);
  }
  .tab button,
  .tab.home {
    border: none;
    background: transparent;
    border-radius: 6px 6px 0 0;
  }
  .tab.home {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 7px 14px;
  }
  .tab.home.active {
    background: var(--bg-raised);
  }
  .logo {
    width: 18px;
    height: 15px;
  }
  .logo path {
    fill: none;
    stroke: var(--accent);
    stroke-width: 58;
    stroke-linejoin: round;
  }
  .logo rect {
    fill: var(--accent);
  }
  .tab-label {
    display: flex;
    align-items: center;
    gap: 7px;
    padding: 7px 4px 7px 12px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .tab-close {
    padding: 2px 9px;
    font-size: 16px;
    line-height: 1;
    color: var(--text-muted);
  }
  .tab-close:hover {
    color: var(--text);
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
    background: var(--text-muted);
  }
  .dot.connecting {
    background: #f2c14e;
  }
  .dot.connected {
    background: var(--accent);
  }
  .dot.error {
    background: var(--danger);
  }

  main {
    flex: 1;
    position: relative;
    min-height: 0;
  }
  .pane {
    position: absolute;
    inset: 0;
  }
  .pane.hidden {
    display: none;
  }
  .reconnect {
    position: absolute;
    right: 20px;
    bottom: 20px;
  }

  .hosts {
    display: flex;
    height: 100%;
  }
  aside {
    width: 200px;
    flex-shrink: 0;
    padding: 14px 10px;
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 2px;
    overflow-y: auto;
  }
  .filter {
    display: flex;
    justify-content: space-between;
    border: none;
    background: transparent;
    text-align: left;
    color: var(--text-muted);
  }
  .filter.on {
    background: var(--bg-hover);
    color: var(--text);
  }
  .filter span {
    font-size: 12px;
    opacity: 0.7;
  }
  section {
    flex: 1;
    padding: 16px 20px;
    overflow-y: auto;
  }
  header {
    display: flex;
    gap: 10px;
    margin-bottom: 8px;
  }
  header input {
    max-width: 360px;
  }
  h3 {
    margin: 18px 0 8px;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-muted);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 10px;
  }
  .card {
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
  }
  .card:hover {
    border-color: #00e5fc66;
  }
  .card-main {
    display: flex;
    align-items: center;
    gap: 12px;
    width: 100%;
    border: none;
    border-radius: 0;
    background: transparent;
    padding: 12px;
    text-align: left;
  }
  .avatar {
    width: 34px;
    height: 34px;
    border-radius: 8px;
    display: grid;
    place-items: center;
    background: var(--accent-soft);
    color: var(--accent);
    font-weight: 700;
    flex-shrink: 0;
  }
  .meta {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .meta strong,
  .meta small {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .meta small {
    color: var(--text-muted);
  }
  .card-actions {
    display: flex;
    justify-content: flex-end;
    border-top: 1px solid var(--border);
    padding: 2px 4px;
  }
  .card-actions button {
    padding: 3px 10px;
    font-size: 12px;
    color: var(--text-muted);
  }
  .card-actions button:hover {
    color: var(--text);
  }
  .card-actions .danger:hover {
    color: var(--danger);
  }
  .empty {
    padding: 60px 0;
    text-align: center;
    color: var(--text-muted);
  }
</style>
