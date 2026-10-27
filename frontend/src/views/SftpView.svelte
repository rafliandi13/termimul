<script lang="ts">
  import { onMount, onDestroy, untrack } from 'svelte'
  import type { vault } from '../../wailsjs/go/models'
  import * as SFTP from '../../wailsjs/go/services/SFTPService'
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
  import FilePane from '../components/FilePane.svelte'
  import type { Progress, Status } from '../lib/types'

  let {
    tabId,
    hostId,
    hosts,
    onstatus,
  }: {
    tabId: string
    hostId: string
    hosts: vault.Host[]
    onstatus: (s: Status) => void
  } = $props()

  // A tab's ID is fixed for its lifetime.
  const leftId = untrack(() => `${tabId}:left`)
  const rightId = untrack(() => `${tabId}:right`)
  const progressEvent = untrack(() => `sftp:progress:${tabId}`)

  let left: FilePane | undefined = $state()
  let right: FilePane | undefined = $state()
  let leftEl: HTMLDivElement | undefined = $state()
  let busy = $state(false)
  let progress = $state<Progress | null>(null)
  let notice = $state('')
  let error = $state('')

  const percent = $derived(progress && progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0)

  const paneById = (id: string) => (id === leftId ? left : right)

  // run executes one transfer at a time and shows its progress and result.
  async function run(label: string, job: () => Promise<unknown>) {
    busy = true
    error = ''
    notice = ''
    try {
      const result = await job()
      if (result !== '' && result !== 0) notice = label.replace('{n}', String(result))
    } catch (err) {
      error = String(err)
    } finally {
      busy = false
      progress = null
    }
  }

  async function copy(fromId: string, toId: string, paths: string[]) {
    const from = paneById(fromId)
    const to = paneById(toId)
    if (!from || !to || paths.length === 0 || !to.isConnected()) return
    await run('{n} file disalin', () => SFTP.Copy(tabId, fromId, paths, toId, to.getCwd()))
    await to.refresh()
  }

  const copyRight = () => copy(leftId, rightId, left?.getSelected() ?? [])
  const copyLeft = () => copy(rightId, leftId, right?.getSelected() ?? [])

  // Files dropped from the OS file manager go to the pane under the cursor.
  export function drop(x: number, y: number, paths: string[]) {
    const r = leftEl?.getBoundingClientRect()
    const onLeft = r && x >= r.left && x <= r.right && y >= r.top && y <= r.bottom
    ;(onLeft ? left : right)?.uploadPaths(paths)
  }

  export function reconnect() {
    left?.reconnect()
    right?.reconnect()
  }

  onMount(() => EventsOn(progressEvent, (p: Progress) => (progress = p)))
  onDestroy(() => EventsOff(progressEvent))
</script>

<section class="sftp">
  <div class="panes">
    <div class="side" bind:this={leftEl}>
      <FilePane
        bind:this={left}
        paneId={leftId}
        progressId={tabId}
        {hosts}
        source="local"
        {busy}
        onreceive={(from, paths) => copy(from, leftId, paths)}
        onrun={run}
      />
    </div>

    <div class="transfer" role="group" aria-label="Transfer antar panel">
      <button title="Salin pilihan kiri ke folder kanan" disabled={busy} onclick={copyRight}>→</button>
      <button title="Salin pilihan kanan ke folder kiri" disabled={busy} onclick={copyLeft}>←</button>
    </div>

    <div class="side">
      <FilePane
        bind:this={right}
        paneId={rightId}
        progressId={tabId}
        {hosts}
        source={hostId}
        {busy}
        {onstatus}
        onreceive={(from, paths) => copy(from, rightId, paths)}
        onrun={run}
      />
    </div>
  </div>

  <footer>
    {#if progress}
      <span class="progress-label">
        {progress.count > 1 ? `${progress.index}/${progress.count} · ` : ''}{progress.name} · {percent}%
      </span>
      <span class="progress" style:--p="{percent}%"></span>
    {:else if busy}
      <span>Memproses…</span>
    {:else if error}
      <span class="error-text">{error}</span>
    {:else if notice}
      <span class="ok">{notice}</span>
    {:else}
      <span class="muted">
        Pilih file lalu klik → / ←, atau tarik ke panel sebelah. Bisa antar server; tarik file dari Explorer untuk upload.
      </span>
    {/if}
  </footer>
</section>

<style>
  .sftp {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    background: var(--bg);
  }
  .panes {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: 1fr auto 1fr;
  }
  .side {
    display: flex;
    min-width: 0;
    min-height: 0;
  }
  .side > :global(*) {
    flex: 1;
  }
  .transfer {
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 8px;
    padding: 0 6px;
    border-left: 1px solid var(--border);
    border-right: 1px solid var(--border);
  }
  .transfer button {
    font-size: 16px;
    padding: 6px 10px;
    color: var(--accent);
  }
  footer {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 8px 12px;
    border-top: 1px solid var(--border);
    font-size: 12px;
    min-height: 34px;
  }
  .muted {
    color: var(--text-muted);
  }
  .ok {
    color: var(--accent);
  }
  .progress-label {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 50%;
  }
  .progress {
    flex: 1;
    height: 6px;
    border-radius: 3px;
    background: linear-gradient(to right, var(--accent) var(--p), var(--bg-hover) var(--p));
  }
</style>
