// web/audio.js
export const audioCtx = new (window.AudioContext||window.webkitAudioContext)();
const bank={};
const files={kick:"kick.wav",snare:"snare.wav",ch:"ch.wav",oh:"oh.wav"};

export async function load() {
  await Promise.all(Object.entries(files).map(async([id,u])=>{
    const buf=await (await fetch(u)).arrayBuffer();
    bank[id]=await audioCtx.decodeAudioData(buf);
  }));
}
export function play(id, when){
  const s=audioCtx.createBufferSource();
  s.buffer=bank[id]; s.connect(audioCtx.destination); s.start(when);
}
await load();
globalThis.play=play;

