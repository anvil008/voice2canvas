const TARGET_SAMPLE_RATE = 16_000;
const TARGET_CHUNK_SAMPLES = 3_200; // 200 ms at 16 kHz

export type MicChunkHandler = (pcm16Le: ArrayBuffer) => void;

function downsampleBuffer(
  input: Float32Array,
  inputSampleRate: number,
  outputSampleRate: number,
): Float32Array {
  if (inputSampleRate === outputSampleRate) {
    return input;
  }

  // This averaging resampler follows Google's Gemini Live browser example.
  const ratio = inputSampleRate / outputSampleRate;
  const output = new Float32Array(Math.round(input.length / ratio));
  let inputOffset = 0;

  for (let outputOffset = 0; outputOffset < output.length; outputOffset += 1) {
    const nextInputOffset = Math.min(input.length, Math.round((outputOffset + 1) * ratio));
    let total = 0;
    let count = 0;
    for (let index = inputOffset; index < nextInputOffset; index += 1) {
      total += input[index];
      count += 1;
    }
    output[outputOffset] = count === 0 ? 0 : total / count;
    inputOffset = nextInputOffset;
  }

  return output;
}

function floatToPcm16(samples: Float32Array): Int16Array {
  const pcm = new Int16Array(samples.length);
  for (let index = 0; index < samples.length; index += 1) {
    const sample = Math.max(-1, Math.min(1, samples[index]));
    pcm[index] = Math.round(sample * (sample < 0 ? 0x8000 : 0x7fff));
  }
  return pcm;
}

function encodeLittleEndian(samples: Int16Array): ArrayBuffer {
  const buffer = new ArrayBuffer(samples.length * Int16Array.BYTES_PER_ELEMENT);
  const view = new DataView(buffer);
  for (let index = 0; index < samples.length; index += 1) {
    view.setInt16(index * Int16Array.BYTES_PER_ELEMENT, samples[index], true);
  }
  return buffer;
}

/** Captures mono microphone input and emits 200 ms, 16 kHz, 16-bit LE PCM chunks. */
export class MicCapture {
  private context: AudioContext | undefined;
  private gain: GainNode | undefined;
  private handler: MicChunkHandler | undefined;
  private mediaStream: MediaStream | undefined;
  private node: AudioWorkletNode | undefined;
  private pending = new Int16Array(TARGET_CHUNK_SAMPLES);
  private pendingLength = 0;
  private source: MediaStreamAudioSourceNode | undefined;
  private workletLoaded = false;

  get running(): boolean {
    return this.mediaStream !== undefined;
  }

  async start(handler: MicChunkHandler): Promise<void> {
    if (this.running) {
      this.handler = handler;
      return;
    }

    this.handler = handler;
    const context = await this.getContext();
    const mediaStream = await navigator.mediaDevices.getUserMedia({
      audio: {
        autoGainControl: true,
        channelCount: 1,
        echoCancellation: true,
        noiseSuppression: true,
      },
    });

    const source = context.createMediaStreamSource(mediaStream);
    const node = new AudioWorkletNode(context, "voice2canvas-capture-processor", {
      channelCount: 1,
      channelCountMode: "explicit",
      numberOfInputs: 1,
      numberOfOutputs: 1,
    });
    const gain = context.createGain();
    gain.gain.value = 0;

    node.port.onmessage = (event: MessageEvent<unknown>) => {
      if (event.data instanceof Float32Array) {
        this.queueFloatSamples(event.data, context.sampleRate);
      }
    };

    source.connect(node);
    // A silent destination keeps the AudioWorklet processing graph alive without feedback.
    node.connect(gain);
    gain.connect(context.destination);

    this.mediaStream = mediaStream;
    this.source = source;
    this.node = node;
    this.gain = gain;
  }

  stop(): void {
    this.flushPending();
    this.node?.disconnect();
    this.source?.disconnect();
    this.gain?.disconnect();
    this.node = undefined;
    this.source = undefined;
    this.gain = undefined;

    this.mediaStream?.getTracks().forEach((track) => track.stop());
    this.mediaStream = undefined;
  }

  async close(): Promise<void> {
    this.stop();
    if (this.context && this.context.state !== "closed") {
      await this.context.close();
    }
    this.context = undefined;
    this.workletLoaded = false;
  }

  private async getContext(): Promise<AudioContext> {
    if (!navigator.mediaDevices?.getUserMedia) {
      if (typeof window !== "undefined" && !window.isSecureContext) {
        throw new Error("Microphone requires a secure HTTPS connection. Please open this page using https://.");
      }
      throw new Error("This browser does not support microphone capture.");
    }

    if (!this.context || this.context.state === "closed") {
      this.context = new AudioContext({ latencyHint: "interactive" });
      this.workletLoaded = false;
    }
    if (!this.workletLoaded) {
      await this.context.audioWorklet.addModule(
        new URL("./capture-processor.js", import.meta.url).href,
      );
      this.workletLoaded = true;
    }
    if (this.context.state === "suspended") {
      await this.context.resume();
    }
    return this.context;
  }

  private queueFloatSamples(samples: Float32Array, sampleRate: number): void {
    const pcm = floatToPcm16(downsampleBuffer(samples, sampleRate, TARGET_SAMPLE_RATE));
    let offset = 0;

    while (offset < pcm.length) {
      const capacity = this.pending.length - this.pendingLength;
      const copied = Math.min(capacity, pcm.length - offset);
      this.pending.set(pcm.subarray(offset, offset + copied), this.pendingLength);
      this.pendingLength += copied;
      offset += copied;

      if (this.pendingLength === this.pending.length) {
        this.handler?.(encodeLittleEndian(this.pending));
        this.pending = new Int16Array(TARGET_CHUNK_SAMPLES);
        this.pendingLength = 0;
      }
    }
  }

  private flushPending(): void {
    if (this.pendingLength > 0) {
      this.handler?.(encodeLittleEndian(this.pending.subarray(0, this.pendingLength)));
      this.pending = new Int16Array(TARGET_CHUNK_SAMPLES);
      this.pendingLength = 0;
    }
  }
}
