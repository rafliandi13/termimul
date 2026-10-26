export type Status = 'connecting' | 'connected' | 'closed' | 'error'

// Payload of "sftp:progress:<id>" events (Progress in sftp.go).
export interface Progress {
  name: string
  done: number
  total: number
  index: number
  count: number
}

export interface Tab {
  id: string
  kind: 'ssh' | 'sftp'
  hostId: string
  title: string
  status: Status
}
