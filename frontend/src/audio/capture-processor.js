// Adapted from Google's Gemini Live browser sample pcm-processor.js.
// It keeps capture work off the main thread; MicCapture performs the required
// sample-rate conversion and PCM framing when each worklet buffer arrives.
class Voice2CanvasCaptureProcessor extends AudioWorkletProcessor {
  constructor() {
    super();
    this.bufferSize = 4096;
    this.buffer = new Float32Array(this.bufferSize);
    this.bufferIndex = 0;
  }

  process(inputs) {
    const input = inputs[0];
    if (!input || input.length === 0 || !input[0]) {
      return true;
    }

    const channelData = input[0];
    for (let index = 0; index < channelData.length; index += 1) {
      this.buffer[this.bufferIndex++] = channelData[index];
      if (this.bufferIndex === this.bufferSize) {
        const completed = this.buffer;
        this.buffer = new Float32Array(this.bufferSize);
        this.bufferIndex = 0;
        this.port.postMessage(completed, [completed.buffer]);
      }
    }

    return true;
  }
}

registerProcessor("voice2canvas-capture-processor", Voice2CanvasCaptureProcessor);
