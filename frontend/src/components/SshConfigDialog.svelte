<script lang="ts">
  import { onMount } from 'svelte'
  import type { services } from '../../wailsjs/go/models'
  import * as Hosts from '../../wailsjs/go/services/HostService'
  import { backdropClose } from '../lib/modal'

  let {
    reveal,
    onimported,
    oncancel,
  }: {
    reveal: boolean
    onimported: () => void
    oncancel: () => void
  } = $props()

  let path = $state('')
  let entries = $state<services.SSHConfigHost[]>([])
  let selected = $state<Record<string, boolean>>({})
  let passphrases = $state<Record<string, string>>({})
  let loading = $state(true)
  let busy = $state(false)
  let error = $state('')
  let result = $state<services.SSHConfigResult | null>(null)

  const count = $derived(entries.filter((h) => selected[h.alias]).length)
  const allOn = $derived(entries.length > 0 && count === entries.length)
  // One passphrase per encrypted key file, however many hosts share it.
  const lockedKeys = $derived([
    ...new Set(entries.filter((h) => selected[h.alias] && h.keyStatus === 'encrypted').map((h) => h.keyPath)),
  ])

  const keyNote: Record<string, string> = {
    ok: 'SSH key',
    encrypted: 'Key terenkripsi',
    missing: 'Key tidak ditemukan',
    invalid: 'Key tidak valid',
  }

  onMount(async () => {
    try {
      const preview = await Hosts.PreviewSSHConfig()
      path = preview.path
      entries = preview.hosts ?? []
      selected = Object.fromEntries(entries.map((h) => [h.alias, !h.duplicate]))
    } catch (err) {
      error = String(err)
    } finally {
      loading = false
    }
  })

  function toggleAll() {
    const on = !allOn
    selected = Object.fromEntries(entries.map((h) => [h.alias, on]))
  }

  const fileName = (p: string) => p.split(/[\\/]/).pop() ?? p

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    const missing = lockedKeys.find((k) => !passphrases[k])
    if (missing) return void (error = `Isi passphrase untuk ${fileName(missing)}`)

    busy = true
    try {
      result = await Hosts.ImportSSHConfig({
        aliases: entries.filter((h) => selected[h.alias]).map((h) => h.alias),
        passphrases,
      })
      onimported()
    } catch (err) {
      error = String(err)
    } finally {
      busy = false
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') oncancel()
  }
</script>

<svelte:window {onkeydown} />

<div class="modal-backdrop" role="presentation" {...backdropClose(oncancel)}>
  <form class="modal" onsubmit={submit}>
    <h2>Import dari SSH config</h2>

    {#if loading}
      <p class="hint">Membaca SSH config…</p>
    {:else if result}
      <p class="notice" role="status">Import selesai.</p>
      <ul class="summary">
        <li><strong>{result.hostsAdded}</strong> host ditambahkan{#if result.hostsSkipped}, {result.hostsSkipped} dilewati karena sudah ada{/if}</li>
        <li><strong>{result.keysAdded}</strong> SSH key ditambahkan ke Keychain</li>
      </ul>
      {#if result.noKey?.length}
        <p class="hint">
          Key untuk {result.noKey.join(', ')} tidak bisa dibaca, jadi host tersebut disimpan dengan login password. Edit
          host untuk mengisi password atau memilih key.
        </p>
      {/if}
    {:else if entries.length > 0}
      <div class="list-head">
        <label class="check">
          <input type="checkbox" checked={allOn} onchange={toggleAll} />
          Pilih semua
        </label>
        <span class="hint">{count} dari {entries.length} dipilih</span>
      </div>
      <ul class="list">
        {#each entries as h (h.alias)}
          <li>
            <label class="check">
              <input type="checkbox" bind:checked={selected[h.alias]} />
              <span class="meta">
                <strong>{h.alias}</strong>
                {#if reveal}<small>{h.username}@{h.address}{h.port !== 22 ? `:${h.port}` : ''}</small>{/if}
              </span>
              <span class="tags">
                {#if h.duplicate}<span class="tag">Sudah ada</span>{/if}
                {#if h.keyStatus}
                  <span class="tag" class:bad={h.keyStatus === 'missing' || h.keyStatus === 'invalid'} title={h.keyError || h.keyPath}>
                    {keyNote[h.keyStatus]}
                  </span>
                {/if}
              </span>
            </label>
          </li>
        {/each}
      </ul>

      {#each lockedKeys as k (k)}
        <label class="field">
          <span>Passphrase untuk {fileName(k)}</span>
          <input type="password" bind:value={passphrases[k]} autocomplete="off" title={k} />
        </label>
      {/each}

      <p class="hint">
        Key disalin terenkripsi ke Keychain. Host tanpa key disimpan dengan login password; isi passwordnya lewat Edit.
      </p>
    {:else if !error}
      <p class="hint">Tidak ada host di <code>{path}</code>. Entri wildcard seperti <code>Host *</code> tidak diimpor.</p>
    {/if}

    {#if error}<p class="error-text">{error}</p>{/if}

    <div class="actions">
      {#if result || (!loading && entries.length === 0)}
        <button type="button" class="primary" onclick={oncancel}>Selesai</button>
      {:else}
        <button type="button" class="ghost" onclick={oncancel}>Batal</button>
        <button type="submit" class="primary" disabled={busy || loading || count === 0}>
          {busy ? 'Mengimpor…' : `Import ${count} host`}
        </button>
      {/if}
    </div>
  </form>
</div>

<style>
  .list-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .list {
    margin: 0;
    padding: 0;
    list-style: none;
    max-height: 300px;
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--bg-input);
  }
  .list li + li {
    border-top: 1px solid var(--border);
  }
  .list .check {
    padding: 8px 10px;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
    cursor: pointer;
  }
  .check input {
    width: auto;
    flex: none;
  }
  .meta {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .meta strong,
  .meta small {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .meta small {
    color: var(--text-muted);
  }
  .tags {
    margin-left: auto;
    display: flex;
    gap: 4px;
    flex: none;
  }
  .tag {
    font-size: 11px;
    padding: 1px 6px;
    border-radius: 4px;
    background: var(--bg-hover);
    color: var(--text-muted);
  }
  .tag.bad {
    color: var(--danger);
  }
  .summary {
    margin: 0;
    padding-left: 18px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  code {
    word-break: break-all;
  }
</style>
