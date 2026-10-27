<script lang="ts">
  import { onMount, onDestroy, untrack, tick } from 'svelte'
  import type { sftpx, vault } from '../../wailsjs/go/models'
  import * as SFTP from '../../wailsjs/go/services/SFTPService'
  import Icon from './Icon.svelte'
  import type { Status } from '../lib/types'

  const LOCAL = 'local'
  const DRAG_TYPE = 'application/x-termimul-files'

  let {
    paneId,
    progressId,
    hosts,
    source: initialSource,
    busy,
    onstatus,
    onreceive,
    onrun,
  }: {
    paneId: string
    progressId: string
    hosts: vault.Host[]
    source: string
    busy: boolean
    onstatus?: (s: Status) => void
    // files dragged in from the other pane
    onreceive: (fromPane: string, paths: string[]) => void
    // runs a transfer through the parent so progress and busy state are shared
    onrun: (label: string, job: () => Promise<unknown>) => Promise<void>
  } = $props()

  let source = $state(untrack(() => initialSource))
  let cwd = $state('')
  let parent = $state('')
  let pathInput = $state('')
  let entries = $state<sftpx.FileEntry[]>([])
  let selected = $state<string[]>([])
  let anchor = -1
  let loading = $state(false)
  let connected = $state(false)
  let error = $state('')
  let renaming = $state<string | null>(null)
  let renameValue = $state('')
  let newFolder = $state<string | null>(null)
  let dragOver = $state(false)
  let renameInput: HTMLInputElement | undefined = $state()
  let folderInput: HTMLInputElement | undefined = $state()

  const isLocal = $derived(source === LOCAL)
  const sourceName = $derived(isLocal ? 'Komputer ini' : (hosts.find((h) => h.id === source)?.label ?? 'Server'))

  async function connect() {
    error = ''
    connected = false
    entries = []
    selected = []
    onstatus?.('connecting')
    try {
      const home = await SFTP.Open(paneId, source)
      connected = true
      onstatus?.('connected')
      await cd(home)
    } catch (err) {
      error = String(err)
      onstatus?.('error')
    }
  }

  async function changeSource(e: Event) {
    source = (e.target as HTMLSelectElement).value
    await SFTP.Close(paneId)
    connect()
  }

  async function cd(dir: string) {
    loading = true
    error = ''
    try {
      const l = await SFTP.List(paneId, dir)
      entries = l.entries
      cwd = l.path
      parent = l.parent
      pathInput = l.path
      selected = []
      anchor = -1
    } catch (err) {
      error = String(err)
      pathInput = cwd
      if (String(err).includes('tertutup')) onstatus?.('closed')
    } finally {
      loading = false
    }
  }

  export function refresh() {
    return cwd ? cd(cwd) : Promise.resolve()
  }
  export const getCwd = () => cwd
  export const getSelected = () => [...selected]
  export const isConnected = () => connected
  export function reconnect() {
    connect()
  }
  export function uploadPaths(paths: string[]) {
    if (!connected || busy || paths.length === 0) return
    onrun(`{n} file disalin ke ${sourceName}`, () => SFTP.UploadPaths(progressId, paneId, cwd, paths)).then(refresh)
  }

  function join(name: string) {
    const sep = cwd.includes('\\') ? '\\' : '/'
    return cwd.endsWith(sep) ? cwd + name : cwd + sep + name
  }

  // Click selects one row, Ctrl/Cmd+click toggles, Shift+click selects a range.
  function select(e: MouseEvent, index: number) {
    const path = entries[index].path
    if (e.shiftKey && anchor >= 0) {
      const [a, b] = [Math.min(anchor, index), Math.max(anchor, index)]
      selected = entries.slice(a, b + 1).map((x) => x.path)
    } else if (e.ctrlKey || e.metaKey) {
      selected = selected.includes(path) ? selected.filter((p) => p !== path) : [...selected, path]
      anchor = index
    } else {
      selected = [path]
      anchor = index
    }
  }

  function open(e: sftpx.FileEntry) {
    if (e.isDir) cd(e.path)
  }

  async function remove(targets: sftpx.FileEntry[]) {
    if (targets.length === 0) return
    const what =
      targets.length === 1
        ? targets[0].isDir && !targets[0].isLink
          ? `folder "${targets[0].name}" beserta seluruh isinya`
          : `"${targets[0].name}"`
        : `${targets.length} item yang dipilih`
    if (!confirm(`Hapus ${what} dari ${sourceName}?\n\nTindakan ini tidak bisa dibatalkan.`)) return
    for (const t of targets) {
      try {
        await SFTP.Remove(paneId, t.path)
      } catch (err) {
        error = String(err)
        break
      }
    }
    await refresh()
  }

  async function startRename(e: sftpx.FileEntry) {
    renaming = e.path
    renameValue = e.name
    await tick()
    renameInput?.select()
  }

  async function commitRename(e: sftpx.FileEntry) {
    const name = renameValue.trim()
    renaming = null
    if (!name || name === e.name) return
    try {
      await SFTP.Rename(paneId, e.path, join(name))
    } catch (err) {
      error = String(err)
    }
    await refresh()
  }

  async function startNewFolder() {
    newFolder = ''
    await tick()
    folderInput?.focus()
  }

  async function commitNewFolder() {
    const name = newFolder?.trim()
    newFolder = null
    if (!name) return
    try {
      await SFTP.MakeDir(paneId, join(name))
    } catch (err) {
      error = String(err)
    }
    await refresh()
  }

  const uploadDialog = () =>
    onrun(`{n} file disalin ke ${sourceName}`, () => SFTP.UploadDialog(progressId, paneId, cwd)).then(refresh)
  const saveAs = (e: sftpx.FileEntry) => onrun('Tersimpan di {n}', () => SFTP.Download(progressId, paneId, e))

  function onkeydown(e: KeyboardEvent, entry: sftpx.FileEntry) {
    if (renaming === entry.path) return
    if (e.key === 'Enter') open(entry)
    if (e.key === 'F2') startRename(entry)
    if (e.key === 'Delete') remove(entries.filter((x) => selected.includes(x.path)))
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'a') {
      e.preventDefault()
      selected = entries.map((x) => x.path)
    }
  }

  // Dragging rows to the other pane copies them there.
  function dragStart(e: DragEvent, entry: sftpx.FileEntry) {
    if (!selected.includes(entry.path)) selected = [entry.path]
    e.dataTransfer?.setData(DRAG_TYPE, JSON.stringify({ pane: paneId, paths: selected }))
    if (e.dataTransfer) e.dataTransfer.effectAllowed = 'copy'
  }

  function dragOverList(e: DragEvent) {
    if (!connected || busy || !e.dataTransfer?.types.includes(DRAG_TYPE)) return
    e.preventDefault()
    e.dataTransfer.dropEffect = 'copy'
    dragOver = true
  }

  function dropOnList(e: DragEvent) {
    dragOver = false
    const raw = e.dataTransfer?.getData(DRAG_TYPE)
    if (!raw) return
    e.preventDefault()
    const { pane, paths } = JSON.parse(raw) as { pane: string; paths: string[] }
    if (pane !== paneId && paths.length) onreceive(pane, paths)
  }

  function size(e: sftpx.FileEntry) {
    if (e.isDir) return ''
    const units = ['B', 'KB', 'MB', 'GB', 'TB']
    let n = e.size
    let i = 0
    while (n >= 1024 && i < units.length - 1) {
      n /= 1024
      i++
    }
    return `${i === 0 ? n : n.toFixed(1)} ${units[i]}`
  }

  const dateFmt = new Intl.DateTimeFormat('id-ID', { dateStyle: 'medium', timeStyle: 'short' })
  const date = (e: sftpx.FileEntry) => dateFmt.format(new Date(e.modTime * 1000))

  onMount(connect)
  onDestroy(() => SFTP.Close(paneId))
</script>

<section class="pane" data-pane={paneId}>
  <div class="bar">
    <select class="source" value={source} onchange={changeSource} aria-label="Sumber file" disabled={busy}>
      <option value={LOCAL}>💻 Komputer ini</option>
      {#each hosts as h (h.id)}<option value={h.id}>{h.label}</option>{/each}
    </select>
    <button class="icon" title="Folder induk" disabled={!parent || loading} onclick={() => cd(parent)}>
      <Icon name="up" />
    </button>
    <button class="icon" title="Muat ulang" disabled={!connected || loading} onclick={refresh}>
      <Icon name="refresh" />
    </button>
    <button class="icon" title="Folder baru" disabled={!connected || busy} onclick={startNewFolder}>
      <Icon name="folder-plus" />
    </button>
    <button class="icon" title="Upload dari file di komputer ini" disabled={!connected || busy} onclick={uploadDialog}>
      <Icon name="upload" />
    </button>
  </div>
  <form class="path" onsubmit={(e) => (e.preventDefault(), cd(pathInput.trim() || cwd))}>
    <input bind:value={pathInput} spellcheck="false" aria-label="Path di {sourceName}" disabled={!connected} />
  </form>

  {#if error}<p class="msg error-text" role="alert">{error}</p>{/if}

  <div
    class="list"
    class:drag-over={dragOver}
    role="grid"
    tabindex="-1"
    aria-busy={loading}
    ondragover={dragOverList}
    ondragleave={() => (dragOver = false)}
    ondrop={dropOnList}
  >
    <div class="row head" role="row">
      <span role="columnheader">Nama</span>
      <span role="columnheader" class="num">Ukuran</span>
      <span role="columnheader">Diubah</span>
      <span></span>
    </div>

    {#if newFolder !== null}
      <div class="row" role="row">
        <span class="name dir">
          <Icon name="folder" />
          <input
            bind:this={folderInput}
            bind:value={newFolder}
            placeholder="Nama folder baru"
            onkeydown={(e) => {
              if (e.key === 'Enter') commitNewFolder()
              if (e.key === 'Escape') newFolder = null
            }}
            onblur={commitNewFolder}
          />
        </span>
      </div>
    {/if}

    {#each entries as e, i (e.path)}
      <div
        class="row"
        class:selected={selected.includes(e.path)}
        role="row"
        tabindex="0"
        draggable={renaming !== e.path}
        ondragstart={(ev) => dragStart(ev, e)}
        onclick={(ev) => select(ev, i)}
        ondblclick={() => open(e)}
        onkeydown={(ev) => onkeydown(ev, e)}
        title={e.mode}
      >
        <span class="name" class:dir={e.isDir}>
          <Icon name={e.isDir ? 'folder' : 'file'} />
          {#if renaming === e.path}
            <input
              bind:this={renameInput}
              bind:value={renameValue}
              onclick={(ev) => ev.stopPropagation()}
              onkeydown={(ev) => {
                ev.stopPropagation()
                if (ev.key === 'Enter') commitRename(e)
                if (ev.key === 'Escape') renaming = null
              }}
              onblur={() => commitRename(e)}
            />
          {:else}
            <span class="label" title={e.name}>{e.name}</span>
            {#if e.isLink}<span class="tag">link</span>{/if}
          {/if}
        </span>
        <span class="num muted">{size(e)}</span>
        <span class="muted">{date(e)}</span>
        <span class="actions">
          {#if !isLocal}
            <button class="icon" title="Simpan ke…" disabled={busy} onclick={(ev) => (ev.stopPropagation(), saveAs(e))}>
              <Icon name="import" size={14} />
            </button>
          {/if}
          <button class="icon" title="Ganti nama (F2)" onclick={(ev) => (ev.stopPropagation(), startRename(e))}>
            <Icon name="edit" size={14} />
          </button>
          <button class="icon danger" title="Hapus (Del)" onclick={(ev) => (ev.stopPropagation(), remove([e]))}>
            <Icon name="trash" size={14} />
          </button>
        </span>
      </div>
    {:else}
      {#if connected && !loading}
        <p class="empty">Folder kosong.</p>
      {:else if !connected && !error}
        <p class="empty">Menghubungkan ke {sourceName}…</p>
      {/if}
    {/each}
  </div>

  <div class="foot muted">
    {selected.length ? `${selected.length} dipilih · ` : ''}{entries.length} item
    {#if !connected && error}
      <button class="link" onclick={connect}>Hubungkan ulang</button>
    {/if}
  </div>
</section>

<style>
  .pane {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    background: var(--bg);
  }
  .bar {
    display: flex;
    gap: 4px;
    align-items: center;
    padding: 8px 8px 4px;
  }
  .source {
    flex: 1;
    min-width: 0;
    font-weight: 600;
  }
  .icon {
    padding: 6px 8px;
    display: inline-grid;
    place-items: center;
  }
  .path {
    padding: 0 8px 8px;
    border-bottom: 1px solid var(--border);
  }
  .path input {
    font-family: var(--font-mono);
    font-size: 12px;
    padding: 5px 8px;
  }
  .msg {
    margin: 6px 8px 0;
  }
  .list {
    flex: 1;
    overflow: auto;
    padding: 2px 0;
    outline: none;
  }
  .list.drag-over {
    box-shadow: inset 0 0 0 2px var(--accent);
    background: var(--accent-soft);
  }
  .row {
    display: grid;
    grid-template-columns: minmax(120px, 1fr) 72px 130px 84px;
    align-items: center;
    gap: 8px;
    padding: 4px 8px;
    cursor: default;
    user-select: none;
  }
  .row:not(.head):hover {
    background: var(--bg-hover);
  }
  .row.selected {
    background: var(--accent-soft);
  }
  .row:focus-visible {
    outline: 1px solid var(--accent);
    outline-offset: -1px;
  }
  .head {
    position: sticky;
    top: 0;
    background: var(--bg);
    font-size: 11px;
    color: var(--text-muted);
    border-bottom: 1px solid var(--border);
    z-index: 1;
  }
  .name {
    display: flex;
    align-items: center;
    gap: 7px;
    min-width: 0;
    color: var(--text-muted);
  }
  .name.dir {
    color: var(--accent);
  }
  .name .label {
    color: var(--text);
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .name input {
    padding: 2px 6px;
  }
  .tag {
    font-size: 10px;
    padding: 0 5px;
    border: 1px solid var(--border);
    border-radius: 4px;
    color: var(--text-muted);
  }
  .num {
    text-align: right;
  }
  .muted {
    color: var(--text-muted);
    font-size: 12px;
    white-space: nowrap;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    opacity: 0;
  }
  .row:hover .actions,
  .row.selected .actions,
  .row:focus-within .actions {
    opacity: 1;
  }
  .actions button {
    border-color: transparent;
    background: transparent;
    color: var(--text-muted);
    padding: 3px 5px;
  }
  .actions button:hover {
    color: var(--text);
    background: var(--bg-raised);
  }
  .actions .danger:hover {
    color: var(--danger);
  }
  .empty {
    padding: 30px 0;
    text-align: center;
    color: var(--text-muted);
  }
  .foot {
    padding: 6px 10px;
    border-top: 1px solid var(--border);
    display: flex;
    gap: 8px;
    align-items: center;
  }
  .link {
    border: none;
    background: none;
    color: var(--accent);
    padding: 0;
    font-size: 12px;
  }
</style>
