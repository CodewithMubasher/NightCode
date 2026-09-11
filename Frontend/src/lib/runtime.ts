import { selectScenario, type ScenarioCallbacks } from "@/lib/scenarios"

export interface RuntimeCallbacks {
  onEvent: ScenarioCallbacks["onEvent"]
}

export function emitFakeRuntime(callbacks: RuntimeCallbacks, message: string) {
  const scenario = selectScenario(message)
  return scenario(callbacks)
}
