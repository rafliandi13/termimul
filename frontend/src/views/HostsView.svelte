<script lang="ts">
  import type { vault } from '../../wailsjs/go/models'
  import BackupDialog from '../components/BackupDialog.svelte'
  import Icon from '../components/Icon.svelte'

  let {
    hosts,
    groupFilter,
    onopen,
    onsftp,
    onedit,
    ondelete,
    onimported,
  }: {
    hosts: vault.Host[]
    groupFilter: string | null
    onopen: (h: vault.Host) => void
    onsftp: (h: vault.Host) => void
    onedit: (h: vault.Host | null) => void
    ondelete: (h: vault.Host) => void
    onimported: () => void
  } = $props()

  let query = $state('')
  let backup = $state<'export' | 'import' | null>(null)

  // Addresses are hidden by default so they don't leak during screen sharing.
  // The choice is a per-device convenience, so localStorage is enough.
  const REVEAL_KEY = 'termimus.revealAddresses'
  let reveal = $state(readReveal())

  function readReveal() {
    try {
      return localStorage.getItem(REVEAL_KEY) === '1'
    } catch {
      return false
    }
  }

  function toggleReveal() {
    reveal = !reveal
    try {
      localStorage.setItem(REVEAL_KEY, reveal ? '1' : '0')
    } catch {
      // storage unavailable: the toggle still works for this session
    }
  }

  const visible = $derived(
    hosts.filter((h) => {
      if (groupFilter !== null && h.group !== groupFilter) return false
      const q = query.trim().toLowerCase()
      return !q || [h.label, h.address, h.username, h.group].some((v) => v.toLowerCase().includes(q))
    }),
  )
  const grouped = $derived.by(() => {
    const map = new Map<string, vault.Host[]>()
    for (const h of visible) {
      const key = h.group || 'Tanpa grup'
      map.set(key, [...(map.get(key) ?? []), h])
    }
    return [...map]
  })
</script>

<section class="page">
  <div class="toolbar">
    <input type="search" placeholder="Cari host, alamat, user…" bind:value={query} />
    <span class="spacer"></span>
    <button onclick={() => (backup = 'import')} title="Import host dari file backup"><Icon name="import" size={14} /> Import</button>
    <button onclick={() => (backup = 'export')} title="Export semua host ke file backup terenkripsi">
      <Icon name="upload" size={14} /> Export
    </button>
    <button
      class="icon-btn"
      onclick={toggleReveal}
      aria-pressed={reveal}
      title={reveal ? 'Sembunyikan alamat & username' : 'Tampilkan alamat & username'}
    >
      <Icon name={reveal ? 'eye' : 'eye-off'} size={16} />
    </button>
    <button class="primary" onclick={() => onedit(null)}><Icon name="plus" size={14} /> Host baru</button>
  </div>

  {#if hosts.length === 0}
    <div class="empty">
      <p>Belum ada host tersimpan.</p>
      <div class="empty-actions">
        <button class="primary" onclick={() => onedit(null)}>Tambah host pertama</button>
        <button onclick={() => (backup = 'import')}>Import dari backup</button>
      </div>
    </div>
  {:else if visible.length === 0}
    <div class="empty"><p>Tidak ada host yang cocok dengan "{query}".</p></div>
  {/if}

  {#each grouped as [group, list] (group)}
    <h3>{group}</h3>
    <div class="grid">
      {#each list as h (h.id)}
        <div class="card">
          <button class="card-body" onclick={() => onopen(h)} title="Hubungkan ke {h.label}">
            <span class="tile" aria-hidden="true">{h.label.slice(0, 1).toUpperCase()}</span>
            <span class="meta">
              <strong>{h.label}</strong>
              {#if reveal}
                <small>{h.username}@{h.address}{h.port !== 22 ? `:${h.port}` : ''}</small>
              {:else}
                <small>{h.authType === 'key' ? 'SSH key' : 'Password'}</small>
              {/if}
            </span>
            {#if h.authType === 'key'}<span class="auth" title="Login dengan SSH key"><Icon name="key" size={13} /></span>{/if}
          </button>
          <div class="card-actions">
            <button onclick={() => onsftp(h)} title="Buka file browser SFTP"><Icon name="folder" size={12} /> SFTP</button>
            <button onclick={() => onedit(h)}>Edit</button>
            <button class="danger" onclick={() => ondelete(h)}>Hapus</button>
          </div>
        </div>
      {/each}
    </div>
  {/each}
</section>

{#if backup}
  <BackupDialog mode={backup} {onimported} oncancel={() => (backup = null)} />
{/if}

<style>
  .toolbar button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .empty-actions {
    display: flex;
    gap: 8px;
    justify-content: center;
  }
  .icon-btn {
    padding: 7px 9px;
    color: var(--text-muted);
  }
  .icon-btn[aria-pressed='true'] {
    color: var(--accent);
  }
  .card-actions button {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }
  .card-actions button:first-child {
    margin-right: auto;
  }
  .auth {
    margin-left: auto;
    color: var(--text-muted);
    display: grid;
  }
</style>
