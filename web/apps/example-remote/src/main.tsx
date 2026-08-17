import { StrictMode } from 'react';
import { createRoot, type Root } from 'react-dom/client';

import { App } from './App';

let root: Root | undefined;

function mount(): void {
  const container = document.getElementById('root');
  if (!container) {
    throw new Error('Remote application root element is missing');
  }
  root ??= createRoot(container);
  root.render(
    <StrictMode>
      <App />
    </StrictMode>,
  );
}

function unmount(): void {
  root?.unmount();
  root = undefined;
}

if (window.__POWERED_BY_WUJIE__) {
  window.__WUJIE_MOUNT = mount;
  window.__WUJIE_UNMOUNT = unmount;
} else {
  mount();
}
