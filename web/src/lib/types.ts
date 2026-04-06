export interface GatewayStatus {
  gatewayId: string;
  name?: string;
  state: string;
  disabled: boolean;
  lastError?: string;
  lastConnectedAt?: string;
  lastVerifiedAt?: string;
}

export interface BootstrapState {
  phase: string;
  setupRequired: boolean;
  sshSession: boolean;
  session: {
    authenticated: boolean;
    trustedLoopback: boolean;
    scope?: string;
    expiresAt?: string;
  };
  config: {
    path: string;
    version: number;
  };
  relay: {
    listenHost: string;
    listenPort: string;
    serverURL: string;
  };
  admin: {
    listenHost: string;
    listenPort: string;
    url: string;
    setupURL?: string;
    setupTokenRequired: boolean;
    setupTokenExpiresAt?: string;
  };
  feishu: {
    appCount: number;
    enabledAppCount: number;
    configuredAppCount: number;
    runtimeConfiguredApps: number;
  };
  gateways?: GatewayStatus[];
}

export interface RuntimeStatus {
  instances: Array<Record<string, unknown>>;
  surfaces: Array<Record<string, unknown>>;
  gateways?: GatewayStatus[];
  pendingRemoteTurns: Array<Record<string, unknown>>;
  activeRemoteTurns: Array<Record<string, unknown>>;
}
