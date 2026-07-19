// Screen Wake Lock, held only while a cooking session is open. Requires a
// secure context (https or localhost) — silently unavailable otherwise;
// use `adb reverse` for real-device evaluation over USB.

let lock: WakeLockSentinel | null = null;
let wanted = false;

async function acquire() {
  try {
    lock = await navigator.wakeLock?.request("screen");
  } catch {
    lock = null; // insecure context or power-save mode: degrade silently
  }
}

export function holdWakeLock() {
  wanted = true;
  void acquire();
  document.addEventListener("visibilitychange", reacquire);
}

export function releaseWakeLock() {
  wanted = false;
  document.removeEventListener("visibilitychange", reacquire);
  void lock?.release();
  lock = null;
}

function reacquire() {
  if (wanted && document.visibilityState === "visible") void acquire();
}
