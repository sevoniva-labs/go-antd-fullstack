import { useCallback, useEffect, useRef } from 'react';

export interface IsolatedIframeRuntimeProps {
  readonly entryUrl: string;
  readonly title: string;
  readonly startupTimeoutMs: number;
  readonly className?: string;
  readonly onReady: () => void;
  readonly onFailure: (error: unknown) => void;
}

export function IsolatedIframeRuntime({
  entryUrl,
  title,
  startupTimeoutMs,
  className,
  onReady,
  onFailure,
}: IsolatedIframeRuntimeProps) {
  const timeoutRef = useRef<number | undefined>(undefined);

  useEffect(() => {
    timeoutRef.current = globalThis.setTimeout(
      () => onFailure(new Error('Isolated iframe startup timed out')),
      startupTimeoutMs,
    );
    return () => {
      if (timeoutRef.current !== undefined) globalThis.clearTimeout(timeoutRef.current);
    };
  }, [onFailure, startupTimeoutMs]);

  const handleLoad = useCallback(() => {
    if (timeoutRef.current !== undefined) {
      globalThis.clearTimeout(timeoutRef.current);
      timeoutRef.current = undefined;
    }
    onReady();
  }, [onReady]);

  return (
    <iframe
      allow="camera 'none'; geolocation 'none'; microphone 'none'; payment 'none'; usb 'none'"
      className={className}
      data-microapp-runtime="iframe"
      loading="eager"
      referrerPolicy="no-referrer"
      sandbox="allow-forms allow-same-origin allow-scripts"
      src={entryUrl}
      title={title}
      onError={() => onFailure(new Error('Isolated iframe failed to load'))}
      onLoad={handleLoad}
    />
  );
}
