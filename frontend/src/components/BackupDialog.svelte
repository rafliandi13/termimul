<script lang="ts">
  import { untrack } from 'svelte'
  import type { services } from '../../wailsjs/go/models'
  import * as Backup from '../../wailsjs/go/services/BackupService'
  import { backdropClose } from '../lib/modal'

  let {
    mode: initialMode,
    onimported,
    oncancel,
  }: {
    mode: 'export' | 'import'
    onimported: () => void
    oncancel: () => void
  } = $props()

  const MIN = 8
  let mode = $state(untrack(() => initialMode))
  let password = $state('')
  let confirmPassword = $state('')
  let filePath = $state('')
  let error = $state('')
  let busy = $state(false)
  let done = $state('')
  let result = $state<services.ImportResult | null>(null)

  const fileName = $derived(filePath.split(/[\\/]/).pop() ?? '')

  function switchMode(m: 'export' | 'import') {
    mode = m
    error = ''
    done = ''
    result = null
  }

  async function browse() {
    const path = await Backup.PickFile()
    if (path) filePath = path
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    if (mode === 'export') {
      if (password.length < MIN) return void (error = `Password minimal ${MIN} karakter`)
      if (password !== confirmPassword) return void (error = 'Konfirmasi password tidak sama')
    } else if (!filePath) {
      return void (error = 'Pilih file backup dulu')
    }

    busy = true
    try {
      if (mode === 'export') {
        const saved = await Backup.Export(password)
        if (saved) done = saved
      } else {
        result = await Backup.Import(filePath, password)
        onimported()
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
    <h2>{mode === 'export' ? 'Export host' : 'Import host'}</h2>

    <div class="segmented" role="group" aria-label="Export atau import">
      <button type="button" class:on={mode === 'export'} aria-pressed={mode === 'export'} onclick={() => switchMode('export')}>
        Export
      </button>
      <button type="button" class:on={mode === 'import'} aria-pressed={mode === 'import'} onclick={() => switchMode('import')}>
        Import
      </button>
    </div>

    {#if mode === 'export'}
      {#if done}
        <p class="notice" role="status">Backup tersimpan di <code>{done}</code></p>
        <p class="hint">Pindahkan file ini ke perangkat lain, lalu buka Termimul di sana dan pilih Import.</p>
      {:else}
        <p class="hint">
          Semua host, password, SSH key di Keychain, dan Known Hosts disimpan dalam satu file yang dikunci dengan password
          di bawah.
        </p>
        <label class="field">
          <span>Password export</span>
          <input type="password" bind:value={password} autocomplete="new-password" placeholder="Minimal {MIN} karakter" />
        </label>
        <label class="field">
          <span>Ulangi password</span>
          <input type="password" bind:value={confirmPassword} autocomplete="new-password" />
        </label>
        <p class="warn">
          Catat password ini. Tanpa password, file backup tidak bisa dibuka oleh siapa pun, termasuk kamu.
        </p>
      {/if}
    {:else if result}
      <p class="notice" role="status">Import selesai.</p>
      <ul class="summary">
        <li><strong>{result.hostsAdded}</strong> host ditambahkan{#if result.hostsSkipped}, {result.hostsSkipped} dilewati karena sudah ada{/if}</li>
        <li><strong>{result.keysAdded}</strong> SSH key ditambahkan{#if result.keysSkipped}, {result.keysSkipped} sudah ada{/if}</li>
        <li><strong>{result.knownHostsAdded}</strong> known host ditambahkan</li>
      </ul>
    {:else}
      <div class="field">
        <span>File backup (.termimul)</span>
        <div class="row">
          <input class="grow" value={fileName} readonly placeholder="Belum dipilih" title={filePath} />
          <button type="button" onclick={browse}>Pilih…</button>
        </div>
      </div>
      <label class="field">
        <span>Password export</span>
        <input type="password" bind:value={password} autocomplete="current-password" />
      </label>
      <p class="hint">Data yang sudah ada tidak ditimpa. Host dan key yang sama akan dilewati.</p>
    {/if}

    {#if error}<p class="error-text">{error}</p>{/if}

    <div class="actions">
      {#if done || result}
        <button type="button" class="primary" onclick={oncancel}>Selesai</button>
      {:else}
        <button type="button" class="ghost" onclick={oncancel}>Batal</button>
        <button type="submit" class="primary" disabled={busy}>
          {#if busy}{mode === 'export' ? 'Mengenkripsi…' : 'Membuka…'}{:else}{mode === 'export' ? 'Export…' : 'Import'}{/if}
        </button>
      {/if}
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
    min-width: 0;
  }
  .warn {
    margin: 0;
    font-size: 12px;
    color: #f2c14e;
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
