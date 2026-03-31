/**
 * Codex app-server protocol constants and classification helpers.
 *
 * The wrapper speaks the Codex App Server protocol: bidirectional JSON-RPC 2.0
 * over stdio with JSONL framing. These constants define the known method names
 * and helpers to classify them.
 */

import type { MessageType } from "./types.js";

/** Known Codex protocol method names grouped by direction and purpose. */
export const CODEX_METHODS = {
  /** Methods sent TO codex (input). */
  input: {
    turnStart: "turn/start",
    turnInterrupt: "turn/interrupt",
    turnSteer: "turn/steer",
  },
  /** Methods received FROM codex (output). */
  output: {
    turnStarted: "turn/started",
    turnCompleted: "turn/completed",
    itemStarted: "item/started",
    itemCompleted: "item/completed",
    agentMessageDelta: "item/agentMessage/delta",
    threadStarted: "thread/started",
  },
  /** Prefix for server request methods (approval requests). */
  serverRequestPrefix: "serverRequest/",
} as const;

/** Item types that indicate tool calls in item/started and item/completed. */
const TOOL_CALL_ITEM_TYPES = new Set([
  "commandExecution",
  "fileChange",
  "dynamicToolCall",
]);

/**
 * Classify a parsed JSONL message from codex stdout based on its `method` field.
 *
 * @param method - The `method` field from the JSON-RPC message, or undefined if absent.
 * @param itemType - Optional item type from the message params (for item/started, item/completed).
 * @returns The classified MessageType.
 */
export function classifyMethod(
  method: string | undefined,
  itemType?: string,
): MessageType {
  if (method === undefined) {
    return "unknown";
  }

  // Agent message streaming
  if (method === CODEX_METHODS.output.agentMessageDelta) {
    return "agentMessage";
  }

  // Item lifecycle — classify based on item type
  if (
    method === CODEX_METHODS.output.itemStarted ||
    method === CODEX_METHODS.output.itemCompleted
  ) {
    if (itemType && TOOL_CALL_ITEM_TYPES.has(itemType)) {
      return "toolCall";
    }
    // item/started or item/completed for agentMessage type
    return "agentMessage";
  }

  // Server requests (approval)
  if (method.startsWith(CODEX_METHODS.serverRequestPrefix)) {
    return "serverRequest";
  }

  // Turn lifecycle
  if (
    method === CODEX_METHODS.output.turnStarted ||
    method === CODEX_METHODS.output.turnCompleted
  ) {
    return "turnLifecycle";
  }

  // Thread lifecycle
  if (method === CODEX_METHODS.output.threadStarted) {
    return "threadLifecycle";
  }

  return "unknown";
}
