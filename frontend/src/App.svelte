<script lang="ts">
  import { onMount } from 'svelte'
  import type { vault } from '../wailsjs/go/models'
  import * as Hosts from '../wailsjs/go/services/HostService'
  import * as Keychain from '../wailsjs/go/services/KeychainService'
  import { OnFileDrop } from '../wailsjs/runtime/runtime'
  import HostForm from './components/HostForm.svelte'
  import Icon from './components/Icon.svelte'
  import HostsView from './views/HostsView.svelte'
  import KeychainView from './views/KeychainView.svelte'
  import KnownHostsView from './views/KnownHostsView.svelte'
  import TunnelsView from './views/TunnelsView.svelte'
  import SftpView from './views/SftpView.svelte'
  import TerminalView from './views/TerminalView.svelte'
  import type { Status, Tab } from './lib/types'

  type Section = 'hosts' | 'keychain' | 'tunnels' | 'knownhosts'

  let hosts = $state<vault.Host[]>([])
  let keys = $state<vault.Key[]>([])
  let tabs = $state<Tab[]>([])
  let activeTab = $state<string | null>(null) // null = home screen
  let section = $state<Section>('hosts')
  let groupFilter = $state<string | null>(null)
  let editing = $state<vault.Host | null | undefined>(undefined) // undefined = form closed, null = new host
  let views: Record<string, TerminalView | SftpView> = {}

  const groups = $derived([...new Set(hosts.map((h) => h.group).filter(Boolean))].sort())

  async function loadHosts() {
    hosts = await Hosts.List()
  }

  async function loadKeys() {
    keys = await Keychain.List()
  }

  function open(h: vault.Host, kind: Tab['kind'] = 'ssh') {
    const id = crypto.randomUUID()
    tabs.push({ id, kind, hostId: h.id, title: h.label, status: 'connecting' })
    activeTab = id
  }

  // Files dropped from Explorer go to the SFTP tab in front, if any.
  function fileDrop(x: number, y: number, paths: string[]) {
    const t = tabs.find((t) => t.id === activeTab)
    if (t?.kind === 'sftp') (views[t.id] as SftpView | undefined)?.drop(x, y, paths)
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

  async function removeHost(h: vault.Host) {
    if (!confirm(`Hapus host "${h.label}"?`)) return
    await Hosts.Delete(h.id)
    await loadHosts()
  }

  async function saved() {
    editing = undefined
    await loadHosts()
  }

  function go(s: Section, group: string | null = null) {
    section = s
    groupFilter = group
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

  onMount(() => {
    loadHosts()
    loadKeys()
    OnFileDrop(fileDrop, false)
  })
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
      Termimul
    </button>
    {#each tabs as t (t.id)}
      <div class="tab" class:active={activeTab === t.id}>
        <button class="tab-label" onclick={() => (activeTab = t.id)} title={t.title}>
          <span class="dot {t.status}" aria-label={t.status}></span>
          <Icon name={t.kind === 'sftp' ? 'folder' : 'terminal'} size={13} />
          {t.title}
        </button>
        <button class="tab-close" aria-label="Tutup tab {t.title}" onclick={() => close(t.id)}>×</button>
      </div>
    {/each}
  </nav>

  <main>
    {#if activeTab === null}
      <div class="home">
        <aside>
          <button class="nav" class:on={section === 'hosts' && groupFilter === null} onclick={() => go('hosts')}>
            <Icon name="hosts" /> Hosts <span class="count">{hosts.length}</span>
          </button>
          {#if section === 'hosts'}
            {#each groups as g}
              <button class="nav sub" class:on={groupFilter === g} onclick={() => go('hosts', g)}>
                {g} <span class="count">{hosts.filter((h) => h.group === g).length}</span>
              </button>
            {/each}
          {/if}
          <button class="nav" class:on={section === 'keychain'} onclick={() => go('keychain')}>
            <Icon name="key" /> Keychain <span class="count">{keys.length}</span>
          </button>
          <button class="nav" class:on={section === 'tunnels'} onclick={() => go('tunnels')}>
            <Icon name="tunnel" /> Port Forwarding
          </button>
          <button class="nav" class:on={section === 'knownhosts'} onclick={() => go('knownhosts')}>
            <Icon name="fingerprint" /> Known Hosts
          </button>
        </aside>

        {#if section === 'hosts'}
          <HostsView
            {hosts}
            {groupFilter}
            onopen={(h) => open(h)}
            onsftp={(h) => open(h, 'sftp')}
            onedit={(h) => (editing = h)}
            ondelete={removeHost}
            onimported={() => (loadHosts(), loadKeys())}
          />
        {:else if section === 'keychain'}
          <KeychainView {keys} onchange={loadKeys} />
        {:else if section === 'tunnels'}
          <TunnelsView {hosts} />
        {:else}
          <KnownHostsView />
        {/if}
      </div>
    {/if}

    {#each tabs as t (t.id)}
      <div class="pane" class:hidden={activeTab !== t.id}>
        {#if t.kind === 'sftp'}
          <SftpView bind:this={views[t.id]} tabId={t.id} hostId={t.hostId} {hosts} onstatus={(s) => setStatus(t.id, s)} />
        {:else}
          <TerminalView
            bind:this={views[t.id]}
            sessionId={t.id}
            hostId={t.hostId}
            active={activeTab === t.id}
            onstatus={(s) => setStatus(t.id, s)}
            onlogout={() => close(t.id)}
          />
        {/if}
        {#if t.status === 'closed' || t.status === 'error'}
          <button class="primary reconnect" onclick={() => views[t.id]?.reconnect()}>Hubungkan ulang</button>
        {/if}
      </div>
    {/each}
  </main>
</div>

{#if editing !== undefined}
  <HostForm
    host={editing}
    {groups}
    {keys}
    onsaved={saved}
    onkeyadded={loadKeys}
    oncancel={() => (editing = undefined)}
  />
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
    gap: 7px;
    padding: 7px 14px;
    font-weight: 600;
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

  .home {
    display: flex;
    height: 100%;
  }
  aside {
    width: 210px;
    flex-shrink: 0;
    padding: 14px 10px;
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 2px;
    overflow-y: auto;
  }
  .nav {
    display: flex;
    align-items: center;
    gap: 10px;
    border: none;
    background: transparent;
    text-align: left;
    color: var(--text-muted);
  }
  .nav:hover {
    color: var(--text);
  }
  .nav.on {
    background: var(--bg-hover);
    color: var(--text);
  }
  .nav.on :global(svg) {
    color: var(--accent);
  }
  .nav.sub {
    padding-left: 40px;
    font-size: 13px;
  }
  .count {
    margin-left: auto;
    font-size: 12px;
    opacity: 0.7;
  }
</style>
