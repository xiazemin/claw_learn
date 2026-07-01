interface OpenSDKOAuth2 {
  authorize(options: {
    appId: string;
    redirect_uri: string;
    scope: string;
    mode: number;
    state?: string;
  }): void;
  Mode: { POPUP: number; REDIRECT: number };
}

interface OpenSDKStatic {
  OAuth2: OpenSDKOAuth2;
  Events: {
    OAuth2Message: string;
    AuthError: string;
  };
  addEventListener(event: string, handler: (event: unknown) => void): void;
  removeEventListener(event: string, handler: (event: unknown) => void): void;
  setDebug(enabled: boolean): void;
  config(options: Record<string, unknown>): Promise<void>;
  create(name: string, options: Record<string, unknown>): unknown;
}

declare const OpenSDK: OpenSDKStatic;

interface Window {
  OpenSDK: OpenSDKStatic;
}
