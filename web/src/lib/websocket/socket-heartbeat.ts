interface SocketHeartbeatOptions {
  intervalMs: number;
  timeoutMs: number;
  isConnected: () => boolean;
  onTimeout: () => void;
  sendPing: () => void;
}

/** 心跳只拥有计时器，不决定连接是否应该重建。 */
export class SocketHeartbeat {
  private intervalId: number | null = null;
  private timeoutId: number | null = null;

  constructor(private readonly options: SocketHeartbeatOptions) {}

  start(): void {
    this.stop();
    if (this.options.intervalMs <= 0) {
      return;
    }
    this.intervalId = window.setInterval(() => {
      this.tick();
    }, this.options.intervalMs);
  }

  acknowledge(): void {
    this.clearTimeout();
  }

  stop(): void {
    if (this.intervalId !== null) {
      window.clearInterval(this.intervalId);
      this.intervalId = null;
    }
    this.clearTimeout();
  }

  private tick(): void {
    if (!this.options.isConnected() || this.timeoutId !== null) {
      return;
    }
    this.options.sendPing();
    this.timeoutId = window.setTimeout(() => {
      this.timeoutId = null;
      this.options.onTimeout();
    }, this.options.timeoutMs);
  }

  private clearTimeout(): void {
    if (this.timeoutId !== null) {
      window.clearTimeout(this.timeoutId);
      this.timeoutId = null;
    }
  }
}
