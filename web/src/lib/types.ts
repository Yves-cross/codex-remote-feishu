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

export interface FeishuAppWizardState {
  credentialsSavedAt?: string;
  connectionVerifiedAt?: string;
  scopesExportedAt?: string;
  eventsConfirmedAt?: string;
  callbacksConfirmedAt?: string;
  menusConfirmedAt?: string;
  publishedAt?: string;
}

export interface FeishuAppSummary {
  id: string;
  name?: string;
  appId?: string;
  hasSecret: boolean;
  enabled: boolean;
  verifiedAt?: string;
  wizard?: FeishuAppWizardState;
  persisted: boolean;
  runtimeOnly?: boolean;
  runtimeOverride?: boolean;
  readOnly?: boolean;
  readOnlyReason?: string;
  status?: GatewayStatus;
}

export interface FeishuAppsResponse {
  apps: FeishuAppSummary[];
}

export interface FeishuAppResponse {
  app: FeishuAppSummary;
}

export interface VerifyResult {
  connected: boolean;
  errorCode?: string;
  errorMessage?: string;
  duration: number;
}

export interface FeishuAppVerifyResponse {
  app: FeishuAppSummary;
  result: VerifyResult;
}

export interface FeishuManifestResponse {
  manifest: FeishuManifest;
}

export interface FeishuManifest {
  scopesImport: {
    scopes: {
      tenant: string[];
      user: string[];
    };
  };
  events: Array<{
    event: string;
    purpose?: string;
  }>;
  menus: Array<{
    key: string;
    name: string;
    description?: string;
  }>;
  checklist: Array<{
    area: string;
    items: string[];
  }>;
}

export interface VSCodeSettingsStatus {
  path: string;
  exists: boolean;
  cliExecutable?: string;
  matchesBinary: boolean;
}

export interface ManagedShimStatus {
  entrypoint: string;
  exists: boolean;
  realBinaryPath?: string;
  realBinaryExists: boolean;
  installed: boolean;
  matchesBinary: boolean;
}

export interface VSCodeDetectResponse {
  sshSession: boolean;
  recommendedMode: string;
  currentMode: string;
  currentBinary: string;
  installStatePath: string;
  installState?: {
    configPath?: string;
    vscodeSettingsPath?: string;
    bundleEntrypoint?: string;
  };
  settings: VSCodeSettingsStatus;
  candidateBundleEntrypoints?: string[];
  latestBundleEntrypoint?: string;
  recordedBundleEntrypoint?: string;
  latestShim: ManagedShimStatus;
  recordedShim?: ManagedShimStatus;
  needsShimReinstall: boolean;
}

export interface SetupCompleteResponse {
  setupRequired: boolean;
  adminURL: string;
  message: string;
}
