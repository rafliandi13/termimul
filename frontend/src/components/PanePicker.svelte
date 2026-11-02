<script lang="ts">
  import type { vault } from '../../wailsjs/go/models'
  import type { SplitDir } from '../lib/panes'
  import { backdropClose } from '../lib/modal'

  let {
    hosts,
    onpick,
    oncancel,
  }: {
    hosts: vault.Host[]
    onpick: (host: vault.Host, dir: SplitDir) => void
    oncancel: () => void
  } = $props()

  let query = $state('')
  let dir = $state<SplitDir>('row')

  const visible = $derived.by(() => {
    const q = query.trim().toLowerCase()
    return hosts.filter((h) => !q || [h.label, h.group].some((v) => v.toLowerCase().includes(q)))
  })

  function onkeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') oncancel()
  }

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (visible.length) onpick(visible[0], dir)
  }
</script>

<svelte:window {onkeydown} />

<div class="modal-backdrop" role="presentation" {...backdropClose(oncancel)}>
  <form class="modal" onsubmit={submit}>
    <h2>Bagi dengan host lain</h2>

    <div class="segmented" role="group" aria-label="Arah pane baru">
      <button type="button" class:on={dir === 'row'} aria-pressed={dir === 'row'} onclick={() => (dir = 'row')}>Ke kanan</button>
      <button type="button" class:on={dir === 'column'} aria-pressed={dir === 'column'} onclick={() => (dir = 'column')}>Ke bawah</button>
    </div>

    <!-- svelte-ignore a11y_autofocus -->
    <input type="search" bind:value={query} placeholder="Cari host…" aria-label="Cari host" autofocus />

    <div class="list">
      {#each visible as h (h.id)}
        <button type="button" onclick={() => onpick(h, dir)}>
          <strong>{h.label}</strong>
          {#if h.group}<small>{h.group}</small>{/if}
        </button>
      {:else}
        <p class="hint">Tidak ada host yang cocok.</p>
      {/each}
    </div>

    <div class="actions">
      <button type="button" class="ghost" onclick={oncancel}>Batal</button>
    </div>
  </form>
</div>

<style>
  .list {
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-height: 280px;
    overflow: auto;
  }
  .list button {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    border-color: transparent;
    background: transparent;
    text-align: left;
  }
  .list button:hover {
    background: var(--bg-hover);
  }
  small {
    color: var(--text-muted);
  }
</style>
