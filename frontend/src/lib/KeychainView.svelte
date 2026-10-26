<script lang="ts">
  import type { main } from '../../wailsjs/go/models'
  import { DeleteKey } from '../../wailsjs/go/main/App'
  import { ClipboardSetText } from '../../wailsjs/runtime/runtime'
  import Icon from './Icon.svelte'
  import KeyDialog from './KeyDialog.svelte'

  let { keys, onchange }: { keys: main.Key[]; onchange: () => void } = $props()

  let dialog = $state<'generate' | 'import' | null>(null)
  let query = $state('')
  let notice = $state('')
  let noticeTimer: ReturnType<typeof setTimeout> | undefined

  const visible = $derived(
    keys.filter((k) => {
      const q = query.trim().toLowerCase()
      return !q || [k.label, k.type, k.fingerprint].some((v) => v.toLowerCase().includes(q))
    }),
  )

  function flash(msg: string) {
    notice = msg
    clearTimeout(noticeTimer)
    noticeTimer = setTimeout(() => (notice = ''), 4000)
  }

  async function copy(k: main.Key) {
    await ClipboardSetText(k.publicKey)
    flash(`Public key "${k.label}" disalin. Tempel ke ~/.ssh/authorized_keys di server.`)
  }

  async function remove(k: main.Key) {
    if (!confirm(`Hapus key "${k.label}"? Tindakan ini tidak bisa dibatalkan.`)) return
    try {
      await DeleteKey(k.id)
      onchange()
    } catch (err) {
      flash(String(err))
    }
  }

  function saved(k: main.Key) {
    dialog = null
    onchange()
    flash(`Key "${k.label}" tersimpan. Klik "Salin public key" untuk memasangnya di server.`)
  }
</script>

<section class="page">
  <div class="toolbar">
    <input type="search" placeholder="Cari key…" bind:value={query} />
    <span class="spacer"></span>
    <button onclick={() => (dialog = 'import')}><Icon name="import" size={14} /> Import</button>
    <button class="primary" onclick={() => (dialog = 'generate')}><Icon name="plus" size={14} /> Key baru</button>
  </div>

  {#if notice}<p class="notice" role="status">{notice}</p>{/if}

  {#if keys.length === 0}
    <div class="empty">
      <p>Belum ada SSH key di keychain.</p>
      <p class="hint">Buat key baru, atau import key yang sudah ada (misalnya dari folder .ssh).</p>
    </div>
  {:else}
    <h3>Keys</h3>
    <div class="grid">
      {#each visible as k (k.id)}
        <div class="card">
          <div class="card-body">
            <span class="tile"><Icon name="key" size={18} /></span>
            <span class="meta">
              <strong>{k.label}</strong>
              <small>{k.type}</small>
              <small class="mono" title={k.fingerprint}>{k.fingerprint}</small>
            </span>
          </div>
          <div class="card-actions">
            <button onclick={() => copy(k)}><Icon name="copy" size={12} /> Salin public key</button>
            <button class="danger" onclick={() => remove(k)}><Icon name="trash" size={12} /> Hapus</button>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>

{#if dialog}
  <KeyDialog mode={dialog} onsaved={saved} oncancel={() => (dialog = null)} />
{/if}

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
</style>
