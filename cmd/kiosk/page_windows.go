//go:build windows

package main

// boot runs before every page the view loads, including the waiting page. It
// reports a heartbeat, so the window can tell a running page from a dead one,
// and it reports how long the screen has been untouched, so a reload lands
// between visitors rather than in front of one.
const boot = `(() => {
  let touched = Date.now();
  for (const e of ['pointerdown', 'keydown', 'wheel', 'touchstart']) {
    addEventListener(e, () => touched = Date.now(), true);
  }
  const beat = () => window.chrome.webview.postMessage(
    'roaster ' + Math.round((Date.now() - touched) / 1000) + ' ' + location.href);
  beat();
  setInterval(beat, 5000);

  // A stand has no right-click and nothing to drag off the screen.
  addEventListener('contextmenu', e => e.preventDefault());
  addEventListener('dragstart', e => e.preventDefault());
})();`

// waiting is what stands on screen until the server answers: a booting device,
// a restart from the hidden menu, or a server that is being restarted under it.
const waiting = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Roaster</title>
<style>
  html, body { height: 100%; margin: 0; background: #070707; color: #fffbf8;
    font-family: system-ui, sans-serif; }
  body { display: grid; place-content: center; justify-items: center; gap: 24px; }
  .dot { width: 14px; height: 14px; border-radius: 50%; background: #0fff50;
    animation: pulse 1.4s cubic-bezier(.45, 0, .15, 1) infinite; }
  p { margin: 0; font-size: 18px; letter-spacing: .01em; }
  small { color: #71717a; font-size: 14px; }
  @keyframes pulse { 0%, 100% { opacity: .25; transform: scale(.85); }
    50% { opacity: 1; transform: scale(1); } }
  @media (prefers-reduced-motion: reduce) { .dot { animation: none; opacity: 1; } }
</style></head>
<body><div class="dot"></div><p>Starting up</p>
<small>This screen comes back by itself.</small></body></html>`
