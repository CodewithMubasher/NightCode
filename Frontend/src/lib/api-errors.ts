import { useSyncExternalStore } from "react"

type Listener = () => void

let currentError: string | null = null
const listeners = new Set<Listener>()

function notify() {
  for (const listener of listeners) listener()
}

/**
 * Records a user-facing API error so mounted pages can render it in an
 * Alert banner instead of swallowing it into the console.
 */
export function reportApiError(message: string) {
  console.error(message)
  currentError = message
  notify()
}

/** Clears the current API error (e.g. when the user dismisses the banner). */
export function clearApiError() {
  if (currentError === null) return
  currentError = null
  notify()
}

function subscribe(listener: Listener) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

function getSnapshot() {
  return currentError
}

/** React hook returning the latest API error (or null). */
export function useApiError(): string | null {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot)
}
