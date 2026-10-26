<script lang="ts">
  import { untrack } from 'svelte'
  import type { main } from '../../wailsjs/go/models'
  import { SaveHost, PickKeyFile } from '../../wailsjs/go/main/App'

  let {
    host,
    groups,
    onsaved,
    oncancel,
  }: {
    host: main.Host | null
    groups: string[]
    onsaved: (h: main.Host) => void
    oncancel: () => void
  } = $props()

  // The form is remounted per edit, so it only needs the host's initial values.
  // Must be a plain object: $state doesn't make class instances (main.HostInput) reactive.
  let form = $state<main.HostInput>(
    untrack(() => ({
      id: host?.id ?? '',
      label: host?.label ?? '',
      group: host?.group ?? '',
      address: host?.address ?? '',
      port: host?.port ?? 22,
      username: host?.username ?? 'root',
      authType: host?.authType ?? 'password',
      keyPath: host?.keyPath ?? '',
      hasPassword: host?.hasPassword ?? false,
      hasPassphrase: host?.hasPassphrase ?? false,
      password: '',
      passphrase: '',
    })),
  )
  let error = $state('')
  let saving = $state(false)

  async function browse() {
    const path = await PickKeyFile()
    if (path) form.keyPath = path
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    saving = true
    try {
      onsaved(await SaveHost({ ...form, port: Number(form.port) || 22 } as main.HostInput))
    } catch (err) {
      error = String(err)
    } finally {
      saving = false
    }
  }

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') oncancel()
  }
</script>

<svelte:window {onkeydown} />

<div class="backdrop" role="presentation" onclick={(e) => e.target === e.currentTarget && oncancel()}>
  <form class="dialog" onsubmit={submit}>
    <h2>{host ? 'Edit host' : 'Host baru'}</h2>

    <label>
      <span>Nama</span>
      <input bind:value={form.label} placeholder="Web server produksi" />
    </label>

    <label>
      <span>Grup</span>
      <input bind:value={form.group} list="groups" placeholder="Production" />
      <datalist id="groups">
        {#each groups as g}<option value={g}></option>{/each}
      </datalist>
    </label>

    <div class="row">
      <label class="grow">
        <span>Alamat</span>
        <input bind:value={form.address} placeholder="192.168.1.10 atau server.domain.com" required />
      </label>
      <label class="port">
        <span>Port</span>
        <input type="number" min="1" max="65535" bind:value={form.port} />
      </label>
    </div>

    <label>
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
      <label>
        <span>Password</span>
        <input
          type="password"
          bind:value={form.password}
          placeholder={host?.hasPassword ? 'Tersimpan. Kosongkan untuk tidak mengubah' : ''}
        />
      </label>
    {:else}
      <label>
        <span>File private key</span>
        <div class="row">
          <input class="grow" bind:value={form.keyPath} placeholder="C:\Users\...\.ssh\id_ed25519" />
          <button type="button" onclick={browse}>Pilih…</button>
        </div>
      </label>
      <label>
        <span>Passphrase (jika ada)</span>
        <input
          type="password"
          bind:value={form.passphrase}
          placeholder={host?.hasPassphrase ? 'Tersimpan. Kosongkan untuk tidak mengubah' : ''}
        />
      </label>
    {/if}

    {#if error}<p class="error">{error}</p>{/if}

    <div class="actions">
      <button type="button" class="ghost" onclick={oncancel}>Batal</button>
      <button type="submit" class="primary" disabled={saving}>{saving ? 'Menyimpan…' : 'Simpan'}</button>
    </div>
  </form>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: #0009;
    display: grid;
    place-items: center;
    z-index: 10;
  }
  .dialog {
    width: min(460px, calc(100vw - 32px));
    max-height: calc(100vh - 32px);
    overflow: auto;
    background: var(--bg-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    padding: 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  h2 {
    margin: 0 0 4px;
    font-size: 17px;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  label > span {
    font-size: 12px;
    color: var(--text-muted);
  }
  .row {
    display: flex;
    gap: 8px;
  }
  .grow {
    flex: 1;
  }
  .port {
    width: 90px;
  }
  .segmented {
    display: flex;
    background: var(--bg-input);
    border: 1px solid var(--border);
    border-radius: 6px;
    padding: 2px;
  }
  .segmented button {
    flex: 1;
    border: none;
    background: transparent;
    padding: 5px;
  }
  .segmented button.on {
    background: var(--bg-hover);
    color: var(--accent);
  }
  .error {
    margin: 0;
    color: var(--danger);
    font-size: 13px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 4px;
  }
</style>
