// Close a dialog on a click that both starts and ends on its backdrop.
// A plain onclick isn't enough: selecting text in an input and releasing the
// mouse over the backdrop fires a click on the backdrop and loses the form.
export function backdropClose(close: () => void) {
  let downOnBackdrop = false
  return {
    onmousedown: (e: MouseEvent) => {
      downOnBackdrop = e.target === e.currentTarget
    },
    onclick: (e: MouseEvent) => {
      if (downOnBackdrop && e.target === e.currentTarget) close()
      downOnBackdrop = false
    },
  }
}
