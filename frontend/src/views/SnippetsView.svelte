<script lang="ts">
  import { onMount } from 'svelte'
  import type { vault } from '../../wailsjs/go/models'
  import * as Snippets from '../../wailsjs/go/services/SnippetService'
  import Icon from '../components/Icon.svelte'
  import { snippetRun, type HostResult } from '../lib/snippetRun.svelte'

  let { hosts }: { hosts: vault.Host[] } = $props()

  let snippets = $state<vault.Snippet[]>([])
  let selectedId = $state<string | null>(null) // null = new snippet
  let name = $state('')
  let command = $state('')
  let picked = $state<Record<string, boolean>>({})
  let error = $state('')
  let notice = $state('')

  // Run settings are a per-device convenience, so localStorage is enough.
  const SETTINGS_KEY = 'termimul.snippetRun'
  const initial = readSettings()
  let concurrency = $state(initial.concurrency)
  let timeoutSec = $state(initial.timeoutSec)

  function readSettings() {
    try {
      const s = JSON.parse(localStorage.getItem(SETTINGS_KEY) ?? '{}')
      return { concurrency: Number(s.concurrency) || 4, timeoutSec: Number(s.timeoutSec ?? 60) || 0 }
    } catch {
      return { concurrency: 4, timeoutSec: 60 }
    }
  }

  function saveSettings() {
    try {
      localStorage.setItem(SETTINGS_KEY, JSON.stringify({ concurrency, timeoutSec }))
    } catch {
      // storage unavailable: the values still apply to this run
    }
  }

  const groups = $derived.by(() => {
    const map = new Map<string, vault.Host[]>()
    for (const h of hosts) {
      const key = h.group || 'Tanpa grup'
      map.set(key, [...(map.get(key) ?? []), h])
    }
    return [...map].sort(([a], [b]) => a.localeCompare(b))
  })
  const chosen = $derived(hosts.filter((h) => picked[h.id]))
  const selected = $derived(snippets.find((s) => s.id === selectedId))
  const dirty = $derived(selected ? name !== selected.name || command !== selected.command : command.trim() !== '')

  const summary = $derived.by(() => {
    const r = snippetRun.results
    const done = r.filter((x) => x.state === 'done')
    return {
      ok: done.filter((x) => x.exitCode === 0).length,
      failed: done.filter((x) => x.exitCode !== 0).length,
      left: r.length - done.length,
    }
  })

  async function load() {
    snippets = await Snippets.List()
  }

  function select(s: vault.Snippet | null) {
    selectedId = s?.id ?? null
    name = s?.name ?? ''
    command = s?.command ?? ''
    error = ''
    notice = ''
  }

  async function save() {
    error = ''
    try {
      const s = await Snippets.Save({ id: selectedId ?? '', name, command })
      await load()
      select(s)
      notice = 'Snippet tersimpan'
    } catch (err) {
      error = String(err)
    }
  }

  async function remove() {
    if (!selected || !confirm(`Hapus snippet "${selected.name}"?`)) return
    await Snippets.Delete(selected.id)
    await load()
    select(null)
  }

  function groupState(list: vault.Host[]) {
    const n = list.filter((h) => picked[h.id]).length
    return { all: n === list.length, some: n > 0 && n < list.length }
  }

  function toggleGroup(list: vault.Host[]) {
    const on = !groupState(list).all
    for (const h of list) picked[h.id] = on
  }

  async function run() {
    error = ''
    notice = ''
    saveSettings()
    try {
      await snippetRun.start(command, chosen, concurrency, timeoutSec)
    } catch (err) {
      error = String(err)
    }
  }

  function status(r: HostResult) {
    if (r.state === 'queued') return 'Menunggu antrean'
    if (r.state === 'running') return 'Berjalan…'
    const time = r.durationMs < 1000 ? `${r.durationMs} ms` : `${(r.durationMs / 1000).toFixed(1)} s`
    return r.error ? `${r.error} · ${time}` : `exit ${r.exitCode} · ${time}`
  }

  function tone(r: HostResult) {
    if (r.state !== 'done') return r.state
    return r.exitCode === 0 ? 'ok' : 'failed'
  }

  onMount(load)
</script>

<section class="page">
  <div class="layout">
    <div class="list">
      <div class="toolbar">
        <h3>Snippets</h3>
        <span class="spacer"></span>
        <button onclick={() => select(null)} title="Snippet baru"><Icon name="plus" size={14} /> Baru</button>
      </div>
      {#each snippets as s (s.id)}
        <button class="item" class:on={s.id === selectedId} onclick={() => select(s)}>
          <strong>{s.name}</strong>
          <small class="mono">{s.command.split('\n')[0]}</small>
        </button>
      {:else}
        <p class="hint">Simpan perintah yang sering dipakai, misalnya <code>df -h</code> atau <code>uptime</code>.</p>
      {/each}
    </div>

    <div class="detail">
      <div class="editor">
        <label class="field">
          <span>Nama</span>
          <input bind:value={name} placeholder="Cek disk" />
        </label>
        <label class="field">
          <span>Perintah</span>
          <textarea class="mono" rows="4" bind:value={command} placeholder="df -h /" spellcheck="false"></textarea>
        </label>
        <div class="row">
          <p class="hint">Dijalankan tanpa terminal interaktif, jadi perintah yang meminta input (mis. sudo dengan password) akan menunggu sampai timeout.</p>
          <span class="spacer"></span>
          {#if selected}<button class="danger" onclick={remove}>Hapus</button>{/if}
          <button onclick={save} disabled={!dirty || !command.trim()}>{selected ? 'Simpan' : 'Simpan snippet'}</button>
        </div>
      </div>

      <h3>Jalankan di</h3>
      {#if hosts.length === 0}
        <p class="hint">Belum ada host tersimpan.</p>
      {:else}
        <div class="hosts">
          {#each groups as [group, list] (group)}
            {@const g = groupState(list)}
            <div class="group">
              <label class="check group-head">
                <input type="checkbox" checked={g.all} indeterminate={g.some} onchange={() => toggleGroup(list)} />
                <strong>{group}</strong>
              </label>
              <div class="chips">
                {#each list as h (h.id)}
                  <label class="check chip" class:on={picked[h.id]}>
                    <input type="checkbox" bind:checked={picked[h.id]} />
                    {h.label}
                  </label>
                {/each}
              </div>
            </div>
          {/each}
        </div>
      {/if}

      <div class="row options">
        <label class="inline">
          Paralel
          <input type="number" min="1" max="64" bind:value={concurrency} />
        </label>
        <label class="inline">
          Timeout (detik)
          <input type="number" min="0" bind:value={timeoutSec} title="0 = tanpa batas" />
        </label>
        <span class="spacer"></span>
        {#if snippetRun.running}
          <button class="danger" onclick={() => snippetRun.cancel()}>Batalkan</button>
        {:else}
          <button class="primary" onclick={run} disabled={!command.trim() || chosen.length === 0}>
            <Icon name="terminal" size={14} /> Jalankan di {chosen.length} host
          </button>
        {/if}
      </div>

      {#if error}<p class="error-text">{error}</p>{/if}
      {#if notice}<p class="notice" role="status">{notice}</p>{/if}

      {#if snippetRun.results.length}
        <h3>
          Hasil
          <span class="tally">
            {summary.ok} berhasil · {summary.failed} gagal{#if summary.left} · {summary.left} belum selesai{/if}
          </span>
        </h3>
        <p class="hint mono last-command" title={snippetRun.command}>$ {snippetRun.command}</p>
        <div class="results">
          {#each snippetRun.results as r (r.hostId)}
            <details open class={tone(r)}>
              <summary>
                <span class="dot"></span>
                <strong>{r.label}</strong>
                <span class="status">{status(r)}</span>
              </summary>
              {#if r.chunks.length}
                <pre>{#each r.chunks as c}<span class={c.stream}>{c.text}</span>{/each}</pre>
              {:else if r.state === 'done'}
                <p class="hint empty-out">Tidak ada output.</p>
              {/if}
            </details>
          {/each}
        </div>
      {/if}
    </div>
  </div>
</section>

<style>
  .layout {
    display: flex;
    gap: 20px;
    align-items: flex-start;
  }
  .list {
    width: 240px;
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .list h3 {
    margin: 0;
  }
  .toolbar button,
  .options button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .item {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    text-align: left;
    border-color: transparent;
    background: transparent;
    min-width: 0;
  }
  .item.on {
    background: var(--bg-hover);
    border-color: var(--border);
  }
  .item strong,
  .item small {
    max-width: 100%;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .item small {
    color: var(--text-muted);
  }
  .detail {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .detail h3 {
    margin: 8px 0 0;
  }
  .editor {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  textarea {
    resize: vertical;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .row .hint {
    flex: 1;
  }
  .spacer {
    flex: 1;
  }
  button.danger {
    color: var(--danger);
  }
  .hosts {
    display: flex;
    flex-direction: column;
    gap: 10px;
    max-height: 260px;
    overflow: auto;
    padding: 10px;
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--bg-input);
  }
  .check {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    cursor: pointer;
  }
  .check input {
    width: auto;
  }
  .group-head {
    margin-bottom: 6px;
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  .chip {
    padding: 4px 10px;
    border: 1px solid var(--border);
    border-radius: 999px;
    background: var(--bg-raised);
  }
  .chip.on {
    border-color: var(--accent);
    color: var(--accent);
  }
  .inline {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 13px;
    color: var(--text-muted);
  }
  .inline input {
    width: 72px;
  }
  .tally {
    margin-left: 8px;
    font-weight: 400;
    text-transform: none;
    letter-spacing: 0;
  }
  .last-command {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .results {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  details {
    border: 1px solid var(--border);
    border-radius: 6px;
    background: var(--bg-raised);
  }
  summary {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    cursor: pointer;
    font-size: 13px;
  }
  summary .status {
    margin-left: auto;
    color: var(--text-muted);
    font-size: 12px;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
    background: var(--text-muted);
  }
  .running .dot {
    background: #f2c14e;
  }
  .ok .dot {
    background: var(--accent);
  }
  .failed .dot {
    background: var(--danger);
  }
  .failed .status {
    color: var(--danger);
  }
  pre {
    margin: 0;
    padding: 10px 12px;
    border-top: 1px solid var(--border);
    background: var(--term-bg);
    font-family: var(--font-mono);
    font-size: 12px;
    max-height: 320px;
    overflow: auto;
    white-space: pre-wrap;
    word-break: break-all;
  }
  pre .stderr {
    color: var(--danger);
  }
  .empty-out {
    padding: 0 12px 10px;
  }
</style>
