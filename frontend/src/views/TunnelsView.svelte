<script lang="ts">
  import { onMount, onDestroy } from 'svelte'
  import type { services, tunnel, vault } from '../../wailsjs/go/models'
  import * as Tunnels from '../../wailsjs/go/services/TunnelService'
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
  import Icon from '../components/Icon.svelte'
  import TunnelForm from '../components/TunnelForm.svelte'

  let { hosts }: { hosts: vault.Host[] } = $props()

  let tunnels = $state<services.TunnelView[]>([])
  let editing = $state<vault.Tunnel | null | undefined>(undefined) // undefined = closed, null = new
  let error = $state('')

  const hostLabel = (id: string) => hosts.find((h) => h.id === id)?.label ?? 'host terhapus'

  async function load() {
    tunnels = await Tunnels.List()
  }

  function route(t: vault.Tunnel) {
    switch (t.type) {
      case 'dynamic':
        return `SOCKS5 di ${t.bindAddress}:${t.bindPort}`
      case 'remote':
        return `server ${t.bindAddress}:${t.bindPort} → ${t.targetHost}:${t.targetPort}`
      default:
        return `${t.bindAddress}:${t.bindPort} → ${t.targetHost}:${t.targetPort}`
    }
  }

  function statusText(s: tunnel.Status) {
    switch (s.state) {
      case 'running':
        return s.connections > 0 ? `Berjalan · ${s.connections} koneksi` : 'Berjalan'
      case 'starting':
        return 'Menghubungkan…'
      case 'error':
        return s.error
      default:
        return 'Berhenti'
    }
  }

  async function toggle(t: services.TunnelView) {
    error = ''
    try {
      if (t.status.state === 'running' || t.status.state === 'starting') await Tunnels.Stop(t.id)
      else await Tunnels.Start(t.id)
    } catch {
      // The error is shown on the card through the status event.
    }
  }

  async function remove(t: services.TunnelView) {
    if (!confirm(`Hapus tunnel "${t.label}"?`)) return
    await Tunnels.Delete(t.id)
    await load()
  }

  async function saved() {
    editing = undefined
    await load()
  }

  onMount(() => {
    EventsOn('tunnel:status', (s: tunnel.Status) => {
      const t = tunnels.find((t) => t.id === s.id)
      if (t) t.status = s
    })
    load()
  })

  onDestroy(() => EventsOff('tunnel:status'))
</script>

<section class="page">
  <div class="toolbar">
    <p class="hint">Teruskan port lewat SSH: akses database, panel admin, atau web internal server dari komputer ini.</p>
    <span class="spacer"></span>
    <button class="primary" onclick={() => (editing = null)} disabled={hosts.length === 0}>
      <Icon name="plus" size={14} /> Tunnel baru
    </button>
  </div>

  {#if error}<p class="error-text">{error}</p>{/if}

  {#if hosts.length === 0}
    <div class="empty"><p>Tambahkan host dulu, lalu buat tunnel lewat host tersebut.</p></div>
  {:else if tunnels.length === 0}
    <div class="empty">
      <p>Belum ada tunnel.</p>
      <p class="hint">Contoh: Local 5432 → localhost:5432 lewat server database, lalu sambungkan aplikasi database ke localhost:5432.</p>
    </div>
  {:else}
    <h3>Tunnels</h3>
    <div class="grid">
      {#each tunnels as t (t.id)}
        <div class="card">
          <div class="card-body">
            <span class="tile" aria-hidden="true">{t.type === 'dynamic' ? 'D' : t.type === 'remote' ? 'R' : 'L'}</span>
            <span class="meta">
              <strong title={t.label}>{t.label}</strong>
              <small class="mono" title={route(t)}>{route(t)}</small>
              <small>via {hostLabel(t.hostId)}{t.autoStart ? ' · otomatis' : ''}</small>
            </span>
          </div>
          <div class="state {t.status.state}" title={t.status.error}>
            <span class="dot"></span>
            <span class="text">{statusText(t.status)}</span>
          </div>
          <div class="card-actions">
            <button class="toggle" onclick={() => toggle(t)}>
              {t.status.state === 'running' || t.status.state === 'starting' ? 'Hentikan' : 'Jalankan'}
            </button>
            <button onclick={() => (editing = t)}>Edit</button>
            <button class="danger" onclick={() => remove(t)}>Hapus</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>

{#if editing !== undefined}
  <TunnelForm tunnel={editing} {hosts} onsaved={saved} oncancel={() => (editing = undefined)} />
{/if}

<style>
  .toolbar .hint {
    margin: 0;
  }
  .toolbar button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }
  .state {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 12px 10px;
    font-size: 12px;
    color: var(--text-muted);
    min-width: 0;
  }
  .state .text {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
    background: var(--text-muted);
  }
  .running .dot {
    background: var(--accent);
  }
  .running {
    color: var(--accent);
  }
  .starting .dot {
    background: #f2c14e;
  }
  .error .dot {
    background: var(--danger);
  }
  .error {
    color: var(--danger);
  }
  .card-actions .toggle {
    margin-right: auto;
    color: var(--accent);
  }
</style>
