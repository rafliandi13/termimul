export type Status = 'connecting' | 'connected' | 'closed' | 'error'

export interface Tab {
  id: string
  hostId: string
  title: string
  status: Status
}
