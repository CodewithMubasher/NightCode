import type { ModelOption } from "@/lib/backend-runtime"

// Home creates a chat and navigates to it before ChatView's first
// startRuntime call happens; this module-level map is a small handoff so the
// model chosen on the home screen carries over to the chat's first message.
// Consumed once (via takePendingModel) so it doesn't leak into later turns.
const pending = new Map<string, ModelOption>()

export function setPendingModel(chatId: string, model: ModelOption | null | undefined) {
  if (model) pending.set(chatId, model)
}

export function takePendingModel(chatId: string): ModelOption | undefined {
  const model = pending.get(chatId)
  pending.delete(chatId)
  return model
}
