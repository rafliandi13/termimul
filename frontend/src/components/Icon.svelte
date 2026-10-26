<script lang="ts">
  let { name, size = 16 }: { name: keyof typeof paths; size?: number } = $props()

  const paths = {
    hosts: 'M4 4h16v7H4zM4 13h16v7H4zM8 7.5h.01M8 16.5h.01',
    key: 'M8 19a4 4 0 1 0 0-8 4 4 0 0 0 0 8ZM10.8 12.2 20 3M16 7l3 3M14 9l2 2',
    fingerprint:
      'M12 11v3c0 2.5-.8 4.8-2.2 6.6M8 9.5a4 4 0 0 1 8 0v3.5c0 1.6-.2 3.1-.7 4.5M4.8 15A10 10 0 0 0 5 12a7 7 0 0 1 12.6-4.2M19 11.8V13c0 2.2-.3 4.3-.9 6.2',
    plus: 'M12 5v14M5 12h14',
    copy: 'M9 9h11v11H9zM5 15H4V4h11v1',
    trash: 'M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3',
    import: 'M12 3v12M7 10l5 5 5-5M5 21h14',
    upload: 'M12 15V3M7 8l5-5 5 5M5 21h14',
    folder: 'M3 6a1 1 0 0 1 1-1h5l2 2h9a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1Z',
    'folder-plus': 'M3 6a1 1 0 0 1 1-1h5l2 2h9a1 1 0 0 1 1 1v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1ZM12 10v6M9 13h6',
    file: 'M6 3h8l4 4v14H6ZM14 3v4h4',
    up: 'M12 19V5M6 11l6-6 6 6',
    refresh: 'M20 11a8 8 0 1 0-2.3 5.7M20 4v7h-7',
    edit: 'M4 20h4L19 9l-4-4L4 16ZM13 7l4 4',
    terminal: 'M4 5h16v14H4zM7 9l3 3-3 3M12 15h5',
    eye: 'M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12ZM12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z',
    'eye-off': 'M3 3l18 18M10.6 5.1A10.7 10.7 0 0 1 12 5c6.4 0 10 7 10 7a17 17 0 0 1-3.2 4.2M6.6 6.6A17 17 0 0 0 2 12s3.6 7 10 7a10 10 0 0 0 5.4-1.6M9.9 9.9a3 3 0 0 0 4.2 4.2',
  }
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="2"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
>
  <path d={paths[name]} />
</svg>
