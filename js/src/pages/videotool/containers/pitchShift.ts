// Loads and wires up the from-scratch pitch-shift AudioWorklet — see js/static/pitch-shift-processor.js for the DSP
// itself (a granular/overlap-add shifter, the same family of technique SoundTouch uses) and why it's a static file
// rather than a blob: URL (this app's CSP has no blob: in script-src and no worklet-src, so a blob worklet module
// gets silently blocked).
const PROCESSOR_URL = "/js/static/pitch-shift-processor.js";

const moduleLoadedFor = new WeakMap<BaseAudioContext, Promise<void>>();

function ensureModuleLoaded(ctx: AudioContext): Promise<void> {
  let p = moduleLoadedFor.get(ctx);
  if (!p) {
    p = ctx.audioWorklet.addModule(PROCESSOR_URL);
    moduleLoadedFor.set(ctx, p);
  }
  return p;
}

export async function createPitchShiftNode(ctx: AudioContext): Promise<AudioWorkletNode> {
  await ensureModuleLoaded(ctx);
  return new AudioWorkletNode(ctx, "pitch-shift-processor", {
    numberOfInputs: 1,
    numberOfOutputs: 1,
    outputChannelCount: [2],
  });
}

export function setPitchSemitones(node: AudioWorkletNode, semitones: number): void {
  const param = node.parameters.get("pitchRatio");
  if (param) param.value = Math.pow(2, semitones / 12);
}
