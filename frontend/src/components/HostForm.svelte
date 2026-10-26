<script lang="ts">
  import { untrack } from 'svelte'
  import type { vault } from '../../wailsjs/go/models'
  import * as Hosts from '../../wailsjs/go/services/HostService'
  import KeyDialog from './KeyDialog.svelte'
  import { backdropClose } from '../lib/modal'

  let {
    host,
    groups,
    keys,
    onsaved,
    onkeyadded,
    oncancel,
  }: {
    host: vault.Host | null
    groups: string[]
    keys: vault.Key[]
    onsaved: (h: vault.Host) => void
    onkeyadded: () => void
    oncancel: () => void
  } = $props()

  // The form is remounted per edit, so it only needs the host's initial values.
  // Must be a plain object: $state doesn't make class instances (vault.HostInput) reactive.
  let form = $state<vault.HostInput>(
    untrack(() => ({
      id: host?.id ?? '',
      label: host?.label ?? '',
      group: host?.group ?? '',
      address: host?.address ?? '',
      port: host?.port ?? 22,
      username: host?.username ?? 'root',
      authType: host?.authType ?? 'password',
      keyId: host?.keyId ?? '',
      keyPath: host?.keyPath ?? '',
      hasPassword: host?.hasPassword ?? false,
      hasPassphrase: host?.hasPassphrase ?? false,
      password: '',
      passphrase: '',
    })),
  )
  let error = $state('')
  let saving = $state(false)
  let keyDialog = $state<'generate' | 'import' | null>(null)

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    saving = true
    try {
      onsaved(await Hosts.Save({ ...form, port: Number(form.port) || 22 } as vault.HostInput))
    } catch (err) {
      error = String(err)
    } finally {
      saving = false
    }
  }

  function keyAdded(k: vault.Key) {
    keyDialog = null
    form.keyId = k.id
    onkeyadded()
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape' && !keyDialog) oncancel()
  }
</script>

<svelte:window {onkeydown} />

<div class="modal-backdrop" role="presentation" {...backdropClose(oncancel)}>
  <form class="modal" onsubmit={submit}>
    <h2>{host ? 'Edit host' : 'Host baru'}</h2>

    <label class="field">
      <span>Nama</span>
      <input bind:value={form.label} placeholder="Web server produksi" />
    </label>

    <label class="field">
      <span>Grup</span>
      <input bind:value={form.group} list="groups" placeholder="Production" />
      <datalist id="groups">
        {#each groups as g}<option value={g}></option>{/each}
      </datalist>
    </label>

    <div class="row">
      <label class="field grow">
        <span>Alamat</span>
        <input bind:value={form.address} placeholder="192.168.1.10 atau server.domain.com" required />
      </label>
      <label class="field port">
        <span>Port</span>
        <input type="number" min="1" max="65535" bind:value={form.port} />
      </label>
    </div>

    <label class="field">
      <span>Username</span>
      <input bind:value={form.username} required />
    </label>

    <div class="segmented" role="group" aria-label="Metode login">
      <button
        type="button"
        class:on={form.authType === 'password'}
        aria-pressed={form.authType === 'password'}
        onclick={() => (form.authType = 'password')}
      >
        Password
      </button>
      <button
        type="button"
        class:on={form.authType === 'key'}
        aria-pressed={form.authType === 'key'}
        onclick={() => (form.authType = 'key')}
      >
        SSH key
      </button>
    </div>

    {#if form.authType === 'password'}
      <label class="field">
        <span>Password</span>
        <input
          type="password"
          bind:value={form.password}
          placeholder={host?.hasPassword ? 'Tersimpan. Kosongkan untuk tidak mengubah' : ''}
        />
      </label>
    {:else}
      <div class="field">
        <span>SSH key dari Keychain</span>
        <div class="row">
          <select class="grow" bind:value={form.keyId}>
            <option value="">{keys.length ? 'Pilih key…' : 'Keychain masih kosong'}</option>
            {#each keys as k (k.id)}<option value={k.id}>{k.label} ({k.type})</option>{/each}
          </select>
          <button type="button" onclick={() => (keyDialog = 'import')}>Import…</button>
          <button type="button" onclick={() => (keyDialog = 'generate')}>Buat…</button>
        </div>
      </div>
      {#if !form.keyId && form.keyPath}
        <p class="hint">
          Saat ini memakai file <code>{form.keyPath}</code> yang tidak bisa dipindah ke keychain otomatis. Pilih atau
          import key untuk menggantinya.
        </p>
      {/if}
    {/if}

    {#if error}<p class="error-text">{error}</p>{/if}

    <div class="actions">
      <button type="button" class="ghost" onclick={oncancel}>Batal</button>
      <button type="submit" class="primary" disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan'}</button>
    </div>
  </form>
</div>

{#if keyDialog}
  <KeyDialog mode={keyDialog} onsaved={keyAdded} oncancel={() => (keyDialog = null)} />
{/if}

<style>
  .row {
    display: flex;
    gap: 8px;
  }
  .grow {
    flex: 1;
    min-width: 0;
  }
  .port {
    width: 90px;
  }
  code {
    word-break: break-all;
  }
</style>
