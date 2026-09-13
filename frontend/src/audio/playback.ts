const PLAYBACK_SAMPLE_RATE = 24_000;

function decodeLittleEndianPcm16(buffer: ArrayBuffer): Float32Array {
  const view = new DataView(buffer);
  const samples = new Float32Array(buffer.byteLength / Int16Array.BYTES_PER_ELEMENT);
  for (let index = 0; index < samples.length; index += 1) {
    samples[index] = view.getInt16(index * Int16Array.BYTES_PER_ELEMENT, true) / 0x8000;
  }
  return samples;
}

/** Schedules server-provided 24 kHz PCM and can cancel all queued speech on interruption. */
export class PcmPlayback {
  private context: AudioContext | undefined;
  private nextStartTime = 0;
  private readonly scheduledSources = new Set<AudioBufferSourceNode>();

  async activate(): Promise<void> {
    const context = this.getContext();
    if (context.state === "suspended") {
      await context.resume();
    }
  }

  async queue(pcm16Le: ArrayBuffer): Promise<void> {
    if (pcm16Le.byteLength === 0) {
      return;
    }
    await this.activate();
    const context = this.getContext();
    const samples = decodeLittleEndianPcm16(pcm16Le);
    const buffer = context.createBuffer(1, samples.length, PLAYBACK_SAMPLE_RATE);
    buffer.getChannelData(0).set(samples);

    const source = context.createBufferSource();
    source.buffer = buffer;
    source.connect(context.destination);
    source.onended = () => this.scheduledSources.delete(source);

    const startTime = Math.max(context.currentTime, this.nextStartTime);
    source.start(startTime);
    this.nextStartTime = startTime + buffer.duration;
    this.scheduledSources.add(source);
  }

  flush(): void {
    for (const source of this.scheduledSources) {
      try {
        source.stop();
      } catch {
        // A source may already have ended between the set iteration and stop().
      }
    }
    this.scheduledSources.clear();
    if (this.context) {
      this.nextStartTime = this.context.currentTime;
    }
  }

  async close(): Promise<void> {
    this.flush();
    if (this.context && this.context.state !== "closed") {
      await this.context.close();
    }
    this.context = undefined;
    this.nextStartTime = 0;
  }

  private getContext(): AudioContext {
    if (!this.context || this.context.state === "closed") {
      this.context = new AudioContext({ latencyHint: "interactive" });
    }
    return this.context;
  }
}
