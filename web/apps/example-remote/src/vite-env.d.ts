/// <reference types="vite/client" />

import type { HostSdk } from '@forge/host-sdk';

declare global {
  interface Window {
    __POWERED_BY_WUJIE__?: boolean;
    __WUJIE_MOUNT?: () => void;
    __WUJIE_UNMOUNT?: () => void;
    $wujie?: {
      props?: {
        hostSdk?: HostSdk;
      };
    };
  }
}

export {};
