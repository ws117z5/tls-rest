// From-scratch real-time pitch shifter (no third-party DSP library) — see
// js/src/pages/videotool/containers/pitchShift.ts for how this is loaded and used. Conceptually the same family of
// technique as SoundTouch's granular/WSOLA approach: audio is buffered into alternating "pages"; once a page is
// full, several overlapping Hann-windowed read pointers ("grains"), evenly spaced across the page, replay the
// *previous* page at a rate scaled by the desired pitch ratio, cross-fading between them (overlap-add) so the
// splice between repeated/skipped stretches of audio isn't audible as a click. Reading faster than real time raises
// pitch (and vice versa) without touching playback tempo, which the <video> element's own playbackRate already
// handles natively (with preservesPitch left on) — this node only adds the pitch offset on top of whatever speed is
// already playing.
//
// Served as a plain static file (not bundled/inlined) because AudioWorkletProcessor code runs in its own worklet
// global scope, loaded via audioWorklet.addModule(url) — a blob: URL would be blocked by this app's CSP, whose
// script-src has no blob: and which has no worklet-src (so worklets fall back to script-src, not worker-src, per
// the CSP spec). A same-origin /js/static/ file sidesteps that entirely.
class PitchShiftProcessor extends AudioWorkletProcessor {
  static get parameterDescriptors() {
    return [{ name: "pitchRatio", defaultValue: 1, minValue: 0.25, maxValue: 4, automationRate: "k-rate" }];
  }

  constructor() {
    super();
    this.pageSize = 4096; // ~85ms at 48kHz: long enough for reasonable low-frequency fidelity, short enough for low latency
    // Number of evenly-spaced overlapping grains (75% overlap at 4). More grains = smoother, less "buzzy" blending
    // between repeated/skipped stretches of audio, at the (here, negligible) cost of a few more additions per
    // sample. For M equally-spaced Hann grains, Σ w[n - k·pageSize/M] (k=0..M-1) is a PROVABLE constant, M/2, at
    // every sample n — a standard trig identity (Σcos(θ - 2πk/M) = 0 for M ≥ 2) — so normalizing by 2/M gives exact
    // unity-gain reconstruction for any M ≥ 2, not an approximation.
    this.grainCount = 4;
    this.maxChannels = 2;
    this.pages = [
      [new Float32Array(this.pageSize), new Float32Array(this.pageSize)],
      [new Float32Array(this.pageSize), new Float32Array(this.pageSize)],
    ];
    this.writePage = 0;
    this.writeIndex = 0;
    this.outputCount = 0; // monotonic output-sample counter; grain phase is derived fresh from this each sample
    // Periodic Hann (divide by pageSize, not pageSize-1): satisfies the constant-overlap-add identity above. The
    // symmetric variant (pageSize-1) does NOT satisfy it, which produced an audible slow amplitude ripple.
    this.hann = new Float32Array(this.pageSize);
    for (let i = 0; i < this.pageSize; i++) {
      this.hann[i] = 0.5 - 0.5 * Math.cos((2 * Math.PI * i) / this.pageSize);
    }
  }

  process(inputs, outputs, parameters) {
    const input = inputs[0];
    const output = outputs[0];
    if (!input || !output || input.length === 0) return true;

    const pitchRatio = parameters.pitchRatio[0];
    const numChannels = Math.min(input.length, output.length, this.maxChannels);
    const blockSize = output[0].length;
    const hop = this.pageSize / this.grainCount;
    const gain = 2 / this.grainCount; // exact COLA normalization — see constructor comment

    for (let i = 0; i < blockSize; i++) {
      const readyPageIdx = this.writePage ^ 1;

      for (let ch = 0; ch < numChannels; ch++) {
        this.pages[ch][this.writePage][this.writeIndex] = input[ch][i];

        const readyPage = this.pages[ch][readyPageIdx];
        let sample = 0;
        for (let g = 0; g < this.grainCount; g++) {
          // Phase recomputed fresh from the running counter each sample (not accumulated per-grain), so the grains
          // can never drift out of their exact even spacing, regardless of pitchRatio or how long playback has run.
          const phase = (this.outputCount + g * hop) * pitchRatio;
          const pos = phase % this.pageSize;
          const idx = Math.floor(pos);
          const frac = pos - idx;
          const next = (idx + 1) % this.pageSize;
          sample += (readyPage[idx] + (readyPage[next] - readyPage[idx]) * frac) * this.hann[idx];
        }
        output[ch][i] = sample * gain;
      }

      this.outputCount++;
      this.writeIndex++;
      if (this.writeIndex >= this.pageSize) {
        this.writeIndex = 0;
        this.writePage ^= 1;
      }
    }
    return true;
  }
}
registerProcessor("pitch-shift-processor", PitchShiftProcessor);
