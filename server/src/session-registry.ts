export type SessionState = "idle" | "executing" | "waitingApproval";

export type MessageDirection = "in" | "out";

export type MessageClassification =
  | "agentMessage"
  | "toolCall"
  | "serverRequest"
  | "turnLifecycle"
  | "threadLifecycle"
  | "unknown";

export interface SessionConnection {
  close: () => void;
  send: (payload: unknown) => boolean;
}

export interface SessionMessage {
  direction?: MessageDirection;
  classification: MessageClassification;
  method?: string;
  raw: string;
  payload?: unknown;
  threadId?: string | null;
  turnId?: string | null;
}

export interface SessionHistoryEntry extends Omit<SessionMessage, "direction"> {
  direction: MessageDirection;
  receivedAt: string;
}

export interface SessionSummary {
  sessionId: string;
  displayName: string;
  state: SessionState;
  online: boolean;
  turnCount: number;
  threadId: string | null;
  turnId: string | null;
  attachedUser: string | null;
  metadata: Record<string, unknown>;
  graceExpiresAt: string | null;
}

export interface SessionDetail extends SessionSummary {
  historySize: number;
  lastMessage: SessionHistoryEntry | null;
}

export interface RegisterSessionInput {
  sessionId: string;
  displayName: string;
  metadata: Record<string, unknown>;
  connection: SessionConnection;
}

export interface RegisterSessionResult {
  resumed: boolean;
  session: SessionDetail;
}

export type SessionDeliveryResult = "ok" | "not_found" | "offline";

export interface AttachUserResult {
  status: "ok" | "not_found" | "offline" | "conflict";
  session?: SessionDetail;
  previousUser?: string | null;
}

export interface DetachUserResult {
  status: "ok" | "not_found";
  session?: SessionDetail;
  previousUser?: string | null;
}

interface SessionRecord {
  sessionId: string;
  displayName: string;
  state: SessionState;
  online: boolean;
  turnCount: number;
  threadId: string | null;
  turnId: string | null;
  attachedUser: string | null;
  metadata: Record<string, unknown>;
  history: SessionHistoryEntry[];
  connection?: SessionConnection;
  graceExpiresAt: string | null;
  graceTimer?: ReturnType<typeof setTimeout>;
}

export interface SessionRegistryOptions {
  gracePeriodMs: number;
  historyLimit: number;
}

export class SessionRegistry {
  private readonly sessions = new Map<string, SessionRecord>();

  private readonly gracePeriodMs: number;

  private readonly historyLimit: number;

  constructor(options: SessionRegistryOptions) {
    this.gracePeriodMs = Math.max(0, options.gracePeriodMs);
    this.historyLimit = Math.max(0, options.historyLimit);
  }

  register(input: RegisterSessionInput): RegisterSessionResult {
    const existing = this.sessions.get(input.sessionId);
    const resumed = existing !== undefined && !existing.online;

    if (existing?.connection && existing.connection !== input.connection) {
      existing.connection.close();
    }

    const session: SessionRecord =
      existing ??
      ({
        sessionId: input.sessionId,
        displayName: input.displayName,
        state: "idle",
        online: true,
        turnCount: 0,
        threadId: null,
        turnId: null,
        attachedUser: null,
        metadata: {},
        history: [],
        graceExpiresAt: null,
      } satisfies SessionRecord);

    this.clearGraceTimer(session);
    session.displayName = input.displayName;
    session.metadata = { ...input.metadata };
    session.online = true;
    session.connection = input.connection;
    session.graceExpiresAt = null;

    this.sessions.set(input.sessionId, session);

    return {
      resumed,
      session: this.toDetail(session),
    };
  }

  disconnect(
    sessionId: string,
    connection?: SessionConnection,
  ): SessionSummary | undefined {
    const session = this.sessions.get(sessionId);
    if (!session) {
      return undefined;
    }

    if (connection && session.connection !== connection) {
      return this.toSummary(session);
    }

    this.clearGraceTimer(session);
    session.connection = undefined;
    session.online = false;
    session.graceExpiresAt = new Date(
      Date.now() + this.gracePeriodMs,
    ).toISOString();
    session.graceTimer = setTimeout(() => {
      this.evict(sessionId);
    }, this.gracePeriodMs);

    return this.toSummary(session);
  }

  recordMessage(
    sessionId: string,
    message: SessionMessage,
  ): SessionHistoryEntry | undefined {
    const session = this.sessions.get(sessionId);
    if (!session) {
      return undefined;
    }

    if (this.shouldIgnoreMessage(session, message)) {
      return undefined;
    }

    if (message.threadId !== undefined) {
      session.threadId = message.threadId;
    }

    if (message.turnId !== undefined) {
      session.turnId = message.turnId;
    }

    const entry: SessionHistoryEntry = {
      ...message,
      direction: message.direction ?? "out",
      threadId: message.threadId ?? session.threadId,
      turnId: message.turnId ?? session.turnId,
      receivedAt: new Date().toISOString(),
    };

    if (this.historyLimit > 0) {
      session.history.push(entry);
      if (session.history.length > this.historyLimit) {
        session.history.splice(0, session.history.length - this.historyLimit);
      }
    }

    this.applyStateTransition(session, entry);
    return entry;
  }

  getSession(sessionId: string): SessionDetail | undefined {
    const session = this.sessions.get(sessionId);
    return session ? this.toDetail(session) : undefined;
  }

  listSessions(): SessionSummary[] {
    return Array.from(this.sessions.values(), (session) =>
      this.toSummary(session),
    );
  }

  getHistory(sessionId: string, limit?: number): SessionHistoryEntry[] {
    const session = this.sessions.get(sessionId);
    if (!session) {
      return [];
    }

    if (limit === undefined) {
      return session.history.slice();
    }

    if (limit <= 0) {
      return [];
    }

    return session.history.slice(-limit);
  }

  getConnection(sessionId: string): SessionConnection | undefined {
    return this.sessions.get(sessionId)?.connection;
  }

  deliverToSession(sessionId: string, payload: unknown): SessionDeliveryResult {
    const session = this.sessions.get(sessionId);
    if (!session) {
      return "not_found";
    }

    if (!session.online || !session.connection) {
      return "offline";
    }

    return session.connection.send(payload) ? "ok" : "offline";
  }

  attachUser(sessionId: string, userId: string): AttachUserResult {
    const session = this.sessions.get(sessionId);
    if (!session) {
      return { status: "not_found" };
    }

    if (!session.online || !session.connection) {
      return {
        status: "offline",
        session: this.toDetail(session),
      };
    }

    if (session.attachedUser && session.attachedUser !== userId) {
      return {
        status: "conflict",
        session: this.toDetail(session),
        previousUser: session.attachedUser,
      };
    }

    const previousUser = session.attachedUser;
    session.attachedUser = userId;

    return {
      status: "ok",
      session: this.toDetail(session),
      previousUser,
    };
  }

  detachUser(sessionId: string): DetachUserResult {
    const session = this.sessions.get(sessionId);
    if (!session) {
      return { status: "not_found" };
    }

    const previousUser = session.attachedUser;
    session.attachedUser = null;

    return {
      status: "ok",
      session: this.toDetail(session),
      previousUser,
    };
  }

  dispose(): void {
    for (const session of this.sessions.values()) {
      this.clearGraceTimer(session);
    }
  }

  private evict(sessionId: string): void {
    const session = this.sessions.get(sessionId);
    if (!session) {
      return;
    }

    this.clearGraceTimer(session);
    this.sessions.delete(sessionId);
  }

  private clearGraceTimer(session: SessionRecord): void {
    if (session.graceTimer) {
      clearTimeout(session.graceTimer);
      session.graceTimer = undefined;
    }
  }

  private applyStateTransition(
    session: SessionRecord,
    message: SessionHistoryEntry,
  ): void {
    if (message.method === "turn/started") {
      session.state = "executing";
      return;
    }

    if (message.classification === "serverRequest") {
      if (session.state === "idle") {
        return;
      }
      session.state = "waitingApproval";
      return;
    }

    if (
      (message.classification === "agentMessage" ||
        message.classification === "toolCall") &&
      session.state !== "waitingApproval"
    ) {
      session.state = "executing";
      return;
    }

    if (message.method === "turn/completed") {
      if (
        session.state === "executing" ||
        session.state === "waitingApproval"
      ) {
        session.turnCount += 1;
      }
      session.state = "idle";
    }
  }

  private shouldIgnoreMessage(
    session: SessionRecord,
    message: SessionMessage,
  ): boolean {
    return message.classification === "serverRequest" && session.state === "idle";
  }

  private toSummary(session: SessionRecord): SessionSummary {
    return {
      sessionId: session.sessionId,
      displayName: session.displayName,
      state: session.state,
      online: session.online,
      turnCount: session.turnCount,
      threadId: session.threadId,
      turnId: session.turnId,
      attachedUser: session.attachedUser,
      metadata: { ...session.metadata },
      graceExpiresAt: session.graceExpiresAt,
    };
  }

  private toDetail(session: SessionRecord): SessionDetail {
    return {
      ...this.toSummary(session),
      historySize: session.history.length,
      lastMessage: session.history.at(-1) ?? null,
    };
  }
}
