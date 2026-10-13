export interface TimerRegistry {
  schedule: (fn: () => void, delay: number) => number
  cancel: (id: number) => void
  cancelAll: () => void
}

// createTimerRegistry tracks pending window.setTimeout handles so a component
// can cancel all of them on unmount without leaking callbacks.
export function createTimerRegistry(): TimerRegistry {
  const pending = new Set<number>()
  return {
    schedule(fn, delay) {
      const id = window.setTimeout(() => {
        pending.delete(id)
        fn()
      }, delay)
      pending.add(id)
      return id
    },
    cancel(id) {
      window.clearTimeout(id)
      pending.delete(id)
    },
    cancelAll() {
      for (const id of pending) window.clearTimeout(id)
      pending.clear()
    },
  }
}
