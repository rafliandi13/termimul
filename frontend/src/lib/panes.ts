// A terminal tab is a binary tree of splits with one SSH session per leaf.
// Everything here is pure: operations return a new tree.

export interface Leaf {
  kind: 'leaf'
  id: string // also the session ID
  hostId: string
}

export interface Split {
  kind: 'split'
  id: string
  dir: SplitDir
  ratio: number // share of the first child, 0..1
  a: PaneNode
  b: PaneNode
}

export type PaneNode = Leaf | Split
export type SplitDir = 'row' | 'column' // row: side by side, column: stacked
export type Move = 'left' | 'right' | 'up' | 'down'

// Fractions of the tab's area.
export interface Rect {
  x: number
  y: number
  w: number
  h: number
}

export interface Divider {
  id: string // the split it resizes
  dir: SplitDir
  ratio: number
  area: Rect // the split's whole area
}

export interface Layout {
  panes: { pane: Leaf; rect: Rect }[]
  dividers: Divider[]
}

const MIN_RATIO = 0.1

export function clampRatio(r: number) {
  return Math.min(1 - MIN_RATIO, Math.max(MIN_RATIO, r))
}

export function leaves(node: PaneNode): Leaf[] {
  return node.kind === 'leaf' ? [node] : [...leaves(node.a), ...leaves(node.b)]
}

// Replace the leaf with a split holding it and the new pane, in that order.
export function splitPane(node: PaneNode, id: string, dir: SplitDir, added: Leaf, splitId: string): PaneNode {
  if (node.kind === 'leaf') {
    return node.id === id ? { kind: 'split', id: splitId, dir, ratio: 0.5, a: node, b: added } : node
  }
  return { ...node, a: splitPane(node.a, id, dir, added, splitId), b: splitPane(node.b, id, dir, added, splitId) }
}

// Remove a leaf; its sibling takes the parent split's place. The last pane can't be removed.
export function removePane(node: PaneNode, id: string): PaneNode {
  if (node.kind === 'leaf') return node
  if (node.a.kind === 'leaf' && node.a.id === id) return node.b
  if (node.b.kind === 'leaf' && node.b.id === id) return node.a
  return { ...node, a: removePane(node.a, id), b: removePane(node.b, id) }
}

export function setRatio(node: PaneNode, splitId: string, ratio: number): PaneNode {
  if (node.kind === 'leaf') return node
  if (node.id === splitId) return { ...node, ratio: clampRatio(ratio) }
  return { ...node, a: setRatio(node.a, splitId, ratio), b: setRatio(node.b, splitId, ratio) }
}

export function computeLayout(root: PaneNode): Layout {
  const out: Layout = { panes: [], dividers: [] }
  const walk = (node: PaneNode, r: Rect) => {
    if (node.kind === 'leaf') {
      out.panes.push({ pane: node, rect: r })
      return
    }
    out.dividers.push({ id: node.id, dir: node.dir, ratio: node.ratio, area: r })
    if (node.dir === 'row') {
      const w = r.w * node.ratio
      walk(node.a, { ...r, w })
      walk(node.b, { ...r, x: r.x + w, w: r.w - w })
    } else {
      const h = r.h * node.ratio
      walk(node.a, { ...r, h })
      walk(node.b, { ...r, y: r.y + h, h: r.h - h })
    }
  }
  walk(root, { x: 0, y: 0, w: 1, h: 1 })
  return out
}

// The pane next to id in a direction: the one sharing the longest edge with it.
export function neighbour(layout: Layout, id: string, move: Move): string | null {
  const from = layout.panes.find((p) => p.pane.id === id)?.rect
  if (!from) return null
  const near = (a: number, b: number) => Math.abs(a - b) < 1e-6
  const overlap = (a: number, al: number, b: number, bl: number) => Math.min(a + al, b + bl) - Math.max(a, b)
  let best: string | null = null
  let longest = 1e-6
  for (const { pane, rect } of layout.panes) {
    const touches =
      (move === 'left' && near(rect.x + rect.w, from.x)) ||
      (move === 'right' && near(rect.x, from.x + from.w)) ||
      (move === 'up' && near(rect.y + rect.h, from.y)) ||
      (move === 'down' && near(rect.y, from.y + from.h))
    if (!touches) continue
    const shared =
      move === 'left' || move === 'right' ? overlap(rect.y, rect.h, from.y, from.h) : overlap(rect.x, rect.w, from.x, from.w)
    if (shared > longest) {
      longest = shared
      best = pane.id
    }
  }
  return best
}

export type PaneAction = 'split-right' | 'split-down' | 'close' | Move

// Pane shortcuts; anything else goes to the shell. Alt+arrows only move focus
// when there is somewhere to go, which is for the caller to decide.
export function paneAction(e: KeyboardEvent): PaneAction | null {
  const key = e.key.toLowerCase()
  if ((e.ctrlKey || e.metaKey) && e.shiftKey && !e.altKey) {
    if (key === 'd') return 'split-right'
    if (key === 'e') return 'split-down'
  }
  if (e.ctrlKey && !e.shiftKey && !e.altKey && !e.metaKey && key === 'w') return 'close'
  if (e.altKey && !e.ctrlKey && !e.shiftKey && !e.metaKey) {
    const moves: Record<string, Move> = { arrowleft: 'left', arrowright: 'right', arrowup: 'up', arrowdown: 'down' }
    return moves[key] ?? null
  }
  return null
}

// What a terminal sends by itself (cursor and device reports, focus and mouse
// events, OSC replies) only makes sense in its own pane, so it isn't broadcast.
export function isTerminalReport(data: string) {
  return /^\x1b(\[[\d;?>]*[Rcn]|\[[IO]$|\[<|\[M|\]|P)/.test(data)
}
