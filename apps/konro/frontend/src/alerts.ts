// Timer alarm output: WebAudio beep + vibration. The AudioContext must be
// created/resumed inside a user gesture (browser autoplay policy), so
// primeAudio() is called from the "start session" tap.

let ctx: AudioContext | null = null;

export function primeAudio() {
  if (!ctx) ctx = new AudioContext();
  if (ctx.state === "suspended") void ctx.resume();
  // inaudible tick fully unlocks some browsers
  const osc = ctx.createOscillator();
  const gain = ctx.createGain();
  gain.gain.value = 0.001;
  osc.connect(gain).connect(ctx.destination);
  osc.start();
  osc.stop(ctx.currentTime + 0.01);
}

export function alarm() {
  if (ctx && ctx.state === "running") {
    for (let i = 0; i < 4; i++) {
      const t = ctx.currentTime + i * 0.45;
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.frequency.value = 1150;
      gain.gain.setValueAtTime(0.5, t);
      gain.gain.exponentialRampToValueAtTime(0.001, t + 0.35);
      osc.connect(gain).connect(ctx.destination);
      osc.start(t);
      osc.stop(t + 0.4);
    }
  }
  navigator.vibrate?.([400, 150, 400, 150, 600]);
}
