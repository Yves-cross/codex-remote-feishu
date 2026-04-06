export interface APIErrorShape {
  error?: {
    code?: string;
    message?: string;
    details?: unknown;
  };
}

export class APIRequestError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly details?: unknown;

  constructor(status: number, message: string, code?: string, details?: unknown) {
    super(message);
    this.name = "APIRequestError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: "same-origin",
    ...init,
    headers: {
      Accept: "application/json",
      ...(init?.headers ?? {}),
    },
  });

  const text = await response.text();
  const contentType = response.headers.get("content-type") ?? "";
  const isJSON = contentType.includes("application/json");

  if (!response.ok) {
    if (isJSON && text) {
      const payload = JSON.parse(text) as APIErrorShape;
      const apiError = payload.error;
      throw new APIRequestError(
        response.status,
        apiError?.message?.trim() || response.statusText,
        apiError?.code?.trim(),
        apiError?.details,
      );
    }
    throw new APIRequestError(response.status, text.trim() || response.statusText);
  }

  if (!isJSON) {
    throw new APIRequestError(response.status, `unexpected response content-type: ${contentType || "unknown"}`);
  }
  return JSON.parse(text) as T;
}

export function formatError(error: unknown): string {
  if (error instanceof APIRequestError) {
    if (error.code) {
      return `${error.code}: ${error.message}`;
    }
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}
