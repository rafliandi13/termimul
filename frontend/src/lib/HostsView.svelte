<script lang="ts">
  import type { main } from '../../wailsjs/go/models'
  import Icon from './Icon.svelte'

  let {
    hosts,
    groupFilter,
    onopen,
    onsftp,
    onedit,
    ondelete,
  }: {
    hosts: main.Host[]
    groupFilter: string | null
    onopen: (h: main.Host) => void
    onsftp: (h: main.Host) => void
    onedit: (h: main.Host | null) => void
    ondelete: (h: main.Host) => void
  } = $props()

  let query = $state('')

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
</script>

<section class="page">
  <div class="toolbar">
    <input type="search" placeholder="Cari host, alamat, user…" bind:value={query} />
    <span class="spacer"></span>
    <button class="primary" onclick={() => onedit(null)}><Icon name="plus" size={14} /> Host baru</button>
  </div>

  {#if hosts.length === 0}
    <div class="empty">
      <p>Belum ada host tersimpan.</p>
      <button class="primary" onclick={() => onedit(null)}>Tambah host pertama</button>
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
              <small>{h.username}@{h.address}{h.port !== 22 ? `:${h.port}` : ''}</small>
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

<style>
  .toolbar button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
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
