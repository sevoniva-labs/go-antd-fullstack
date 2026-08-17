// @vitest-environment jsdom

import { act, fireEvent, render } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { IsolatedIframeRuntime } from './IsolatedIframe';

afterEach(() => {
  vi.useRealTimers();
});

describe('IsolatedIframeRuntime', () => {
  it('clears the startup timer after a successful load', () => {
    vi.useFakeTimers();
    const onReady = vi.fn();
    const onFailure = vi.fn();
    const screen = render(
      <IsolatedIframeRuntime
        entryUrl="https://partner.example.cn/apps/risk/index.html"
        title="合作方风险应用"
        startupTimeoutMs={5_000}
        onFailure={onFailure}
        onReady={onReady}
      />,
    );

    const iframe = screen.getByTitle('合作方风险应用');
    expect(iframe.getAttribute('sandbox')).toBe('allow-forms allow-same-origin allow-scripts');
    expect(iframe.getAttribute('referrerpolicy')).toBe('no-referrer');
    fireEvent.load(iframe);
    act(() => vi.advanceTimersByTime(5_001));

    expect(onReady).toHaveBeenCalledOnce();
    expect(onFailure).not.toHaveBeenCalled();
  });

  it('fails closed when the isolated app never loads', () => {
    vi.useFakeTimers();
    const onFailure = vi.fn();
    render(
      <IsolatedIframeRuntime
        entryUrl="https://partner.example.cn/apps/risk/index.html"
        title="合作方风险应用"
        startupTimeoutMs={5_000}
        onFailure={onFailure}
        onReady={vi.fn()}
      />,
    );

    act(() => vi.advanceTimersByTime(5_001));
    expect(onFailure).toHaveBeenCalledOnce();
  });
});
