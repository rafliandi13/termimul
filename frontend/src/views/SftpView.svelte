<script lang="ts">
  import { onMount, onDestroy, untrack, tick } from 'svelte'
  import type { sftpx } from '../../wailsjs/go/models'
  import * as SFTP from '../../wailsjs/go/services/SFTPService'
  import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
  import Icon from '../components/Icon.svelte'
  import type { Progress, Status } from '../lib/types'

  let {
    sessionId,
    hostId,
    onstatus,
  }: {
    sessionId: string
    hostId: string
    onstatus: (s: Status) => void
  } = $props()

  // A tab's sessionId is fixed for its lifetime.
  const progressEvent = untrack(() => `sftp:progress:${sessionId}`)

  let cwd = $state('')
  let pathInput = $state('')
  let entries = $state<sftpx.FileEntry[]>([])
  let selected = $state<string | null>(null)
  let loading = $state(false)
  let connected = $state(false)
  let error = $state('')
  let notice = $state('')
  let busy = $state(false)
  let progress = $state<Progress | null>(null)
  let renaming = $state<string | null>(null)
  let renameValue = $state('')
  let newFolder = $state<string | null>(null)
  let renameInput: HTMLInputElement | undefined = $state()
  let folderInput: HTMLInputElement | undefined = $state()

  const parent = $derived(cwd === '/' ? null : cwd.replace(/\/[^/]+\/?$/, '') || '/')

  async function connect() {
    error = ''
    connected = false
    onstatus('connecting')
    try {
      const home = await SFTP.Open(sessionId, hostId)
      connected = true
      onstatus('connected')
      await cd(home)
    } catch (err) {
      error = String(err)
      onstatus('error')
    }
  }

  async function cd(dir: string) {
    loading = true
    error = ''
    try {
      entries = await SFTP.List(sessionId, dir)
      cwd = dir
      pathInput = dir
      selected = null
    } catch (err) {
      error = String(err)
      pathInput = cwd
      if (String(err).includes('tertutup')) onstatus('closed')
    } finally {
      loading = false
    }
  }

  const refresh = () => cd(cwd)

  function join(name: string) {
    return cwd.endsWith('/') ? cwd + name : `${cwd}/${name}`
  }

  async function transfer(label: string, run: () => Promise<unknown>) {
    busy = true
    error = ''
    notice = ''
    try {
      const result = await run()
      if (result !== '' && result !== 0) notice = label.replace('{n}', String(result))
      await refresh()
    } catch (err) {
      error = String(err)
      await refresh()
    } finally {
      busy = false
      progress = null
    }
  }

  const upload = () => transfer('{n} file di-upload ke ' + cwd, () => SFTP.UploadDialog(sessionId, cwd))

  export function drop(paths: string[]) {
    if (!connected || busy || paths.length === 0) return
    transfer('{n} file di-upload ke ' + cwd, () => SFTP.UploadPaths(sessionId, cwd, paths))
  }

  const download = (e: sftpx.FileEntry) => transfer('Tersimpan di {n}', () => SFTP.Download(sessionId, e))

  async function remove(e: sftpx.FileEntry) {
    const what = e.isDir && !e.isLink ? `folder "${e.name}" beserta seluruh isinya` : `"${e.name}"`
    if (!confirm(`Hapus ${what} dari server?\n\nTindakan ini tidak bisa dibatalkan.`)) return
    try {
      await SFTP.Remove(sessionId, e.path)
    } catch (err) {
      error = String(err)
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
      await SFTP.Rename(sessionId, e.path, join(name))
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
      await SFTP.MakeDir(sessionId, join(name))
    } catch (err) {
      error = String(err)
    }
    await refresh()
  }

  function open(e: sftpx.FileEntry) {
    if (e.isDir) cd(e.path)
    else download(e)
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

  const percent = $derived(progress && progress.total > 0 ? Math.round((progress.done / progress.total) * 100) : 0)

  export function reconnect() {
    connect()
  }

  onMount(() => {
    EventsOn(progressEvent, (p: Progress) => (progress = p))
    connect()
  })

  onDestroy(() => {
    EventsOff(progressEvent)
    SFTP.Close(sessionId)
  })
</script>

<section class="sftp">
  <div class="bar">
    <button class="icon" title="Folder induk" disabled={!parent || loading} onclick={() => parent && cd(parent)}>
      <Icon name="up" />
    </button>
    <button class="icon" title="Muat ulang" disabled={!connected || loading} onclick={refresh}>
      <Icon name="refresh" />
    </button>
    <form class="path" onsubmit={(e) => (e.preventDefault(), cd(pathInput.trim() || '/'))}>
      <input bind:value={pathInput} spellcheck="false" aria-label="Path di server" disabled={!connected} />
    </form>
    <button disabled={!connected || busy} onclick={startNewFolder}><Icon name="folder-plus" size={14} /> Folder baru</button>
    <button class="primary" disabled={!connected || busy} onclick={upload}><Icon name="upload" size={14} /> Upload</button>
  </div>

  {#if error}<p class="msg error-text" role="alert">{error}</p>{/if}
  {#if notice}<p class="msg notice" role="status">{notice}</p>{/if}

  <div class="list" role="grid" aria-busy={loading}>
    <div class="row head" role="row">
      <span role="columnheader">Nama</span>
      <span role="columnheader" class="num">Ukuran</span>
      <span role="columnheader">Diubah</span>
      <span role="columnheader">Izin</span>
      <span></span>
    </div>

    {#if newFolder !== null}
      <div class="row" role="row">
        <span class="name">
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

    {#each entries as e (e.path)}
      <div
        class="row"
        class:selected={selected === e.path}
        role="row"
        tabindex="0"
        onclick={() => (selected = e.path)}
        ondblclick={() => open(e)}
        onkeydown={(ev) => {
          if (ev.key === 'Enter' && renaming !== e.path) open(e)
          if (ev.key === 'Delete') remove(e)
          if (ev.key === 'F2') startRename(e)
        }}
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
        <span class="muted mono">{e.mode}</span>
        <span class="actions">
          <button class="icon" title="Download" disabled={busy} onclick={(ev) => (ev.stopPropagation(), download(e))}>
            <Icon name="import" size={14} />
          </button>
          <button class="icon" title="Ganti nama (F2)" onclick={(ev) => (ev.stopPropagation(), startRename(e))}>
            <Icon name="edit" size={14} />
          </button>
          <button class="icon danger" title="Hapus (Del)" onclick={(ev) => (ev.stopPropagation(), remove(e))}>
            <Icon name="trash" size={14} />
          </button>
        </span>
      </div>
    {:else}
      {#if connected && !loading}
        <p class="empty">Folder kosong. Tarik file dari Explorer ke sini, atau klik Upload.</p>
      {:else if !connected && !error}
        <p class="empty">Menghubungkan…</p>
      {/if}
    {/each}
  </div>

  <footer>
    {#if progress}
      <span class="progress-label">
        {progress.count > 1 ? `${progress.index}/${progress.count} · ` : ''}{progress.name} · {percent}%
      </span>
      <span class="progress" style:--p="{percent}%"></span>
    {:else if busy}
      <span class="progress-label">Memproses…</span>
    {:else}
      <span class="muted">{entries.length} item · tarik file dari Explorer untuk upload</span>
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
  .bar {
    display: flex;
    gap: 6px;
    align-items: center;
    padding: 10px 12px;
    border-bottom: 1px solid var(--border);
  }
  .bar button {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    white-space: nowrap;
  }
  .icon {
    padding: 6px 8px;
    display: inline-grid;
    place-items: center;
  }
  .path {
    flex: 1;
    min-width: 0;
  }
  .path input {
    font-family: var(--font-mono);
    font-size: 13px;
  }
  .msg {
    margin: 8px 12px 0;
  }
  .list {
    flex: 1;
    overflow: auto;
    padding: 4px 0;
  }
  .row {
    display: grid;
    grid-template-columns: minmax(200px, 1fr) 90px 170px 110px 110px;
    align-items: center;
    gap: 12px;
    padding: 5px 12px;
    border-radius: 0;
    cursor: default;
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
    font-size: 12px;
    color: var(--text-muted);
    border-bottom: 1px solid var(--border);
    z-index: 1;
  }
  .name {
    display: flex;
    align-items: center;
    gap: 8px;
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
  .mono {
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 2px;
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
    padding: 4px 6px;
  }
  .actions button:hover {
    color: var(--text);
    background: var(--bg-raised);
  }
  .actions .danger:hover {
    color: var(--danger);
  }
  .empty {
    padding: 40px 0;
    text-align: center;
    color: var(--text-muted);
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
