<script lang="ts">
  import { onMount } from 'svelte'
  import type { main } from '../../wailsjs/go/models'
  import { ListKnownHosts, DeleteKnownHost, ImportKnownHosts } from '../../wailsjs/go/main/App'
  import Icon from './Icon.svelte'

  let entries = $state<main.KnownHost[]>([])
  let query = $state('')
  let notice = $state('')
  let importing = $state(false)

  const visible = $derived(
    entries.filter((e) => {
      const q = query.trim().toLowerCase()
      return !q || [e.hosts, e.keyType, e.fingerprint].some((v) => v.toLowerCase().includes(q))
    }),
  )

  async function load() {
    entries = await ListKnownHosts()
  }

  async function importFromOpenSSH() {
    importing = true
    try {
      const n = await ImportKnownHosts()
      notice = n > 0 ? `${n} entri ditambahkan dari ~/.ssh/known_hosts.` : 'Semua entri dari ~/.ssh/known_hosts sudah ada.'
      await load()
    } catch (err) {
      notice = String(err)
    } finally {
      importing = false
    }
  }

  async function remove(e: main.KnownHost) {
    if (!confirm(`Hapus fingerprint ${e.hosts}?\n\nKoneksi berikutnya ke host ini akan meminta konfirmasi fingerprint lagi.`)) return
    await DeleteKnownHost(e.id)
    await load()
  }

  onMount(load)
</script>

<section class="page">
  <div class="toolbar">
    <input type="search" placeholder="Cari host atau fingerprint…" bind:value={query} />
    <span class="spacer"></span>
    <button onclick={importFromOpenSSH} disabled={importing} title="Ambil dari ~/.ssh/known_hosts">
      <Icon name="import" size={14} /> {importing ? 'Mengimpor…' : 'Import'}
    </button>
  </div>

  {#if notice}<p class="notice" role="status">{notice}</p>{/if}

  {#if entries.length === 0}
    <div class="empty">
      <p>Belum ada known host.</p>
      <p class="hint">Fingerprint server tersimpan otomatis saat pertama kali terhubung, atau klik Import untuk mengambil dari OpenSSH.</p>
    </div>
  {:else}
    <h3>Known Hosts <span class="count">{entries.length}</span></h3>
    <div class="grid">
      {#each visible as e (e.id)}
        <div class="card">
          <div class="card-body">
            <span class="tile"><Icon name="fingerprint" size={18} /></span>
            <span class="meta">
              <strong title={e.hosts}>{e.hosts}</strong>
              <small>{e.keyType}</small>
              <small class="mono" title={e.fingerprint}>{e.fingerprint}</small>
            </span>
          </div>
          <div class="card-actions">
            <button class="danger" onclick={() => remove(e)}><Icon name="trash" size={12} /> Hapus</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>

<style>
  .toolbar button,
  .card-actions button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .count {
    font-weight: 400;
    opacity: 0.7;
  }
</style>
