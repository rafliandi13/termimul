<script lang="ts">
  import { untrack } from 'svelte'
  import type { vault } from '../../wailsjs/go/models'
  import * as Tunnels from '../../wailsjs/go/services/TunnelService'
  import { backdropClose } from '../lib/modal'

  let {
    tunnel,
    hosts,
    onsaved,
    oncancel,
  }: {
    tunnel: vault.Tunnel | null
    hosts: vault.Host[]
    onsaved: () => void
    oncancel: () => void
  } = $props()

  let form = $state<vault.Tunnel>(
    untrack(() => ({
      id: tunnel?.id ?? '',
      label: tunnel?.label ?? '',
      hostId: tunnel?.hostId ?? hosts[0]?.id ?? '',
      type: tunnel?.type ?? 'local',
      bindAddress: tunnel?.bindAddress ?? '127.0.0.1',
      bindPort: tunnel?.bindPort ?? 0,
      targetHost: tunnel?.targetHost ?? 'localhost',
      targetPort: tunnel?.targetPort ?? 0,
      autoStart: tunnel?.autoStart ?? false,
    })),
  )
  let error = $state('')
  let saving = $state(false)

  const types = [
    {
      id: 'local',
      name: 'Local',
      help: 'Buka layanan di server (atau jaringannya) dari komputer ini. Contoh: database server di localhost:5432.',
    },
    {
      id: 'remote',
      name: 'Remote',
      help: 'Buka layanan di komputer ini dari sisi server. Contoh: tunjukkan web lokal ke server.',
    },
    {
      id: 'dynamic',
      name: 'SOCKS',
      help: 'Proxy SOCKS5 di komputer ini; semua koneksi lewat server. Atur di browser sebagai proxy SOCKS5.',
    },
  ]

  const exposed = $derived(!['127.0.0.1', 'localhost', '::1', ''].includes(form.bindAddress.trim()))

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    error = ''
    saving = true
    try {
      await Tunnels.Save({ ...form, bindPort: Number(form.bindPort), targetPort: Number(form.targetPort) } as vault.Tunnel)
      onsaved()
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

<div class="modal-backdrop" role="presentation" {...backdropClose(oncancel)}>
  <form class="modal" onsubmit={submit}>
    <h2>{tunnel ? 'Edit tunnel' : 'Tunnel baru'}</h2>

    <div class="segmented" role="group" aria-label="Jenis tunnel">
      {#each types as t}
        <button type="button" class:on={form.type === t.id} aria-pressed={form.type === t.id} onclick={() => (form.type = t.id)}>
          {t.name}
        </button>
      {/each}
    </div>
    <p class="hint">{types.find((t) => t.id === form.type)?.help}</p>

    <label class="field">
      <span>Nama</span>
      <input bind:value={form.label} placeholder="Database produksi" />
    </label>

    <label class="field">
      <span>Lewat host</span>
      <select bind:value={form.hostId} required>
        {#if hosts.length === 0}<option value="">Belum ada host</option>{/if}
        {#each hosts as h (h.id)}<option value={h.id}>{h.label}</option>{/each}
      </select>
    </label>

    <div class="row">
      <label class="field grow">
        <span>{form.type === 'remote' ? 'Alamat listen di server' : 'Alamat listen di komputer ini'}</span>
        <input bind:value={form.bindAddress} placeholder="127.0.0.1" />
      </label>
      <label class="field port">
        <span>Port</span>
        <input type="number" min="1" max="65535" bind:value={form.bindPort} required />
      </label>
    </div>
    {#if exposed}
      <p class="warn">Alamat ini membuat port bisa diakses perangkat lain di jaringan. Pakai 127.0.0.1 kalau hanya untuk diri sendiri.</p>
    {/if}

    {#if form.type !== 'dynamic'}
      <div class="row">
        <label class="field grow">
          <span>{form.type === 'remote' ? 'Tujuan (dari komputer ini)' : 'Tujuan (dari sisi server)'}</span>
          <input bind:value={form.targetHost} placeholder="localhost" required />
        </label>
        <label class="field port">
          <span>Port</span>
          <input type="number" min="1" max="65535" bind:value={form.targetPort} required />
        </label>
      </div>
    {/if}

    <label class="check">
      <input type="checkbox" bind:checked={form.autoStart} />
      Jalankan otomatis saat Termimul dibuka
    </label>

    {#if error}<p class="error-text">{error}</p>{/if}

    <div class="actions">
      <button type="button" class="ghost" onclick={oncancel}>Batal</button>
      <button type="submit" class="primary" disabled={saving || hosts.length === 0}>{saving ? 'Menyimpan…' : 'Simpan'}</button>
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
  .port {
    width: 100px;
  }
  .warn {
    margin: 0;
    font-size: 12px;
    color: #f2c14e;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 13px;
  }
  .check input {
    width: auto;
  }
</style>
