<script lang="ts">
  import { untrack } from 'svelte'
  import type { vault } from '../../wailsjs/go/models'
  import * as Keychain from '../../wailsjs/go/services/KeychainService'
  import { backdropClose } from '../lib/modal'

  let {
    mode: initialMode,
    onsaved,
    oncancel,
  }: {
    mode: 'generate' | 'import'
    onsaved: (k: vault.Key) => void
    oncancel: () => void
  } = $props()

  let mode = $state(untrack(() => initialMode))
  let label = $state('')
  let kind = $state('ed25519')
  let filePath = $state('')
  let text = $state('')
  let passphrase = $state('')
  let error = $state('')
  let busy = $state(false)

  const kinds = [
    { id: 'ed25519', name: 'ED25519', note: 'Disarankan: cepat, kecil, aman' },
    { id: 'rsa', name: 'RSA 4096', note: 'Untuk server lama yang belum mendukung ED25519' },
    { id: 'ecdsa', name: 'ECDSA 256', note: 'Alternatif modern' },
  ]

  async function browse() {
    const path = await Keychain.PickFile()
    if (path) {
      filePath = path
      text = ''
    }
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    busy = true
    try {
      if (mode === 'generate') {
        onsaved(await Keychain.Generate(label || 'key', kind))
      } else if (filePath) {
        onsaved(await Keychain.ImportFile(filePath, label, passphrase))
      } else if (text.trim()) {
        onsaved(await Keychain.ImportText(label || 'imported', text, passphrase))
      } else {
        error = 'Pilih file key atau tempel isinya'
      }
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
    <h2>{mode === 'generate' ? 'Buat key baru' : 'Import key'}</h2>

    <div class="segmented" role="group" aria-label="Cara menambah key">
      <button type="button" class:on={mode === 'generate'} aria-pressed={mode === 'generate'} onclick={() => (mode = 'generate')}>
        Buat baru
      </button>
      <button type="button" class:on={mode === 'import'} aria-pressed={mode === 'import'} onclick={() => (mode = 'import')}>
        Import
      </button>
    </div>

    <label class="field">
      <span>Nama</span>
      <input bind:value={label} placeholder={mode === 'generate' ? 'laptop-kantor' : 'Kosongkan untuk pakai nama file'} />
    </label>

    {#if mode === 'generate'}
      <label class="field">
        <span>Jenis</span>
        <select bind:value={kind}>
          {#each kinds as k}<option value={k.id}>{k.name}</option>{/each}
        </select>
      </label>
      <p class="hint">{kinds.find((k) => k.id === kind)?.note}</p>
    {:else}
      <div class="field">
        <span>File private key</span>
        <div class="row">
          <input class="grow" value={filePath} readonly placeholder="Belum dipilih" />
          <button type="button" onclick={browse}>Pilih…</button>
        </div>
      </div>
      {#if !filePath}
        <label class="field">
          <span>Atau tempel isi private key</span>
          <textarea rows="5" bind:value={text} placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"></textarea>
        </label>
      {/if}
      <label class="field">
        <span>Passphrase (jika key terenkripsi)</span>
        <input type="password" bind:value={passphrase} />
      </label>
      <p class="hint">Key disimpan terenkripsi di Termimus, jadi file aslinya boleh dipindah atau dihapus.</p>
    {/if}

    {#if error}<p class="error-text">{error}</p>{/if}

    <div class="actions">
      <button type="button" class="ghost" onclick={oncancel}>Batal</button>
      <button type="submit" class="primary" disabled={busy}>
        {#if busy}{mode === 'generate' ? 'Membuat…' : 'Mengimpor…'}{:else}{mode === 'generate' ? 'Buat key' : 'Import'}{/if}
      </button>
    </div>
  </form>
</div>

<style>
  .row {
    display: flex;
    gap: 8px;
  }
  .grow {
    flex: 1;
  }
</style>
