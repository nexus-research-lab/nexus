import Matter from "matter-js";

import type { SpotlightToken } from "@/types/app/launcher";

import type { TokenPhysicsConfig } from "./launcher-agent-pile-model";

type TokenKind = SpotlightToken["kind"];

interface LauncherPilePhysicsOptions {
  configs: TokenPhysicsConfig[];
  container: HTMLElement;
  tokenByKey: ReadonlyMap<string, SpotlightToken>;
  tokenRefs: {
    current: Record<string, HTMLElement | null>;
  };
}

interface TokenRenderSnapshot {
  opacity: string;
  transform: string;
  zIndex: string;
}

const ANGULAR_VELOCITY_SCALE: Readonly<Record<TokenKind, number>> = {
  agent: 0.8,
  room: 1.2,
};

export class LauncherPilePhysics {
  private readonly bodyByKey = new Map<string, Matter.Body>();
  private readonly configs: TokenPhysicsConfig[];
  private readonly container: HTMLElement;
  private readonly engine: Matter.Engine;
  private readonly observer: IntersectionObserver;
  private readonly renderByKey = new Map<string, TokenRenderSnapshot>();
  private readonly timeoutIds: number[] = [];
  private readonly tokenByKey: ReadonlyMap<string, SpotlightToken>;
  private readonly tokenRefs: LauncherPilePhysicsOptions["tokenRefs"];
  private animationFrame = 0;
  private disposed = false;
  private documentVisible = document.visibilityState !== "hidden";
  private inView = true;
  private previousTime = performance.now();

  constructor({
    configs,
    container,
    tokenByKey,
    tokenRefs,
  }: LauncherPilePhysicsOptions) {
    this.configs = configs;
    this.container = container;
    this.tokenByKey = tokenByKey;
    this.tokenRefs = tokenRefs;
    this.engine = this.createEngine();
    this.addBounds();
    this.scheduleTokenBodies();
    this.observer = new IntersectionObserver(this.handleIntersection, {
      threshold: 0.05,
    });
    this.observer.observe(container);
    document.addEventListener("visibilitychange", this.handleVisibilityChange);
    this.syncAnimationState();
  }

  dispose(): void {
    this.disposed = true;
    this.timeoutIds.forEach((timeoutId) => window.clearTimeout(timeoutId));
    this.stopAnimation();
    this.observer.disconnect();
    document.removeEventListener("visibilitychange", this.handleVisibilityChange);
    Matter.World.clear(this.engine.world, false);
    Matter.Engine.clear(this.engine);
  }

  private readonly handleIntersection = ([entry]: IntersectionObserverEntry[]) => {
    this.inView = entry?.isIntersecting ?? true;
    this.syncAnimationState();
  };

  private readonly handleVisibilityChange = () => {
    this.documentVisible = document.visibilityState !== "hidden";
    this.syncAnimationState();
  };

  private readonly update = (time: number) => {
    if (this.disposed || !this.documentVisible || !this.inView) {
      this.animationFrame = 0;
      this.previousTime = time;
      return;
    }

    // 限制积分步长，避免页面恢复可见时刚体因超大 delta 穿透边界。
    const delta = Math.min(time - this.previousTime, 1000 / 60);
    this.previousTime = time;
    Matter.Engine.update(this.engine, delta || 1000 / 60);

    const dynamicBodies = this.engine.world.bodies.filter((body) => !body.isStatic);
    const allAsleep = dynamicBodies.length > 0
      && dynamicBodies.every((body) => body.isSleeping);
    const dirty = this.renderTokens();
    if (allAsleep && !dirty) {
      this.animationFrame = 0;
      return;
    }
    this.animationFrame = window.requestAnimationFrame(this.update);
  };

  private createEngine(): Matter.Engine {
    return Matter.Engine.create({
      enableSleeping: true,
      gravity: { x: 0, y: 1.16, scale: 0.0034 },
      positionIterations: 8,
      velocityIterations: 6,
    });
  }

  private addBounds(): void {
    const width = this.container.clientWidth || 560;
    const height = this.container.clientHeight;
    const bounds = [
      Matter.Bodies.rectangle(width / 2, height - 18, width + 120, 28, {
        friction: 0.84,
        isStatic: true,
        restitution: 0.16,
      }),
      Matter.Bodies.rectangle(-18, height / 2, 36, height * 2, {
        friction: 0.9,
        isStatic: true,
        restitution: 0.12,
      }),
      Matter.Bodies.rectangle(width + 18, height / 2, 36, height * 2, {
        friction: 0.9,
        isStatic: true,
        restitution: 0.12,
      }),
      Matter.Bodies.rectangle(-42, height / 2, 180, height * 2, {
        angle: -0.16,
        friction: 0.88,
        isStatic: true,
        restitution: 0.1,
      }),
      Matter.Bodies.rectangle(width + 42, height / 2, 180, height * 2, {
        angle: 0.16,
        friction: 0.88,
        isStatic: true,
        restitution: 0.1,
      }),
    ];
    Matter.World.add(this.engine.world, bounds);
  }

  private scheduleTokenBodies(): void {
    this.configs.forEach((config) => {
      const token = this.tokenByKey.get(config.key);
      if (!token) {
        return;
      }
      const body = this.createTokenBody(config, token.kind);
      this.bodyByKey.set(config.key, body);
      const timeoutId = window.setTimeout(() => {
        Matter.World.add(this.engine.world, body);
      }, config.delay);
      this.timeoutIds.push(timeoutId);
    });
  }

  private createTokenBody(
    config: TokenPhysicsConfig,
    kind: TokenKind,
  ): Matter.Body {
    const common = {
      density: 0.0014,
      friction: 0.22,
      frictionAir: 0.012,
      restitution: 0.18,
      sleepThreshold: 24,
      slop: 0.5,
    };
    const factories: Readonly<Record<TokenKind, () => Matter.Body>> = {
      agent: () => Matter.Bodies.circle(
        config.spawnX,
        config.spawnY,
        config.size / 2,
        common,
      ),
      room: () => Matter.Bodies.rectangle(
        config.spawnX,
        config.spawnY,
        config.size,
        config.size,
        { ...common, chamfer: { radius: config.radius } },
      ),
    };
    const body = factories[kind]();
    Matter.Body.setAngle(body, config.angle);
    Matter.Body.setVelocity(body, {
      x: randomLauncherVelocity(-1.3, 1.3),
      y: randomLauncherVelocity(3.8, 5.6),
    });
    Matter.Body.setAngularVelocity(
      body,
      randomLauncherVelocity(-0.03, 0.03) * ANGULAR_VELOCITY_SCALE[kind],
    );
    return body;
  }

  private renderTokens(): boolean {
    let dirty = false;
    this.configs.forEach((config) => {
      const element = this.tokenRefs.current[config.key];
      const body = this.bodyByKey.get(config.key);
      if (!element || !body) {
        return;
      }
      const snapshot = createRenderSnapshot(body, config);
      const previous = this.renderByKey.get(config.key);
      if (previous && snapshotsMatch(previous, snapshot)) {
        return;
      }
      dirty = true;
      element.style.opacity = snapshot.opacity;
      element.style.transform = snapshot.transform;
      element.style.zIndex = snapshot.zIndex;
      this.renderByKey.set(config.key, snapshot);
    });
    return dirty;
  }

  private startAnimation(): void {
    if (
      this.disposed
      || this.animationFrame !== 0
      || !this.documentVisible
      || !this.inView
    ) {
      return;
    }
    this.previousTime = performance.now();
    this.animationFrame = window.requestAnimationFrame(this.update);
  }

  private stopAnimation(): void {
    if (this.animationFrame === 0) {
      return;
    }
    window.cancelAnimationFrame(this.animationFrame);
    this.animationFrame = 0;
  }

  private syncAnimationState(): void {
    if (this.documentVisible && this.inView) {
      this.startAnimation();
      return;
    }
    this.stopAnimation();
  }
}

function createRenderSnapshot(
  body: Matter.Body,
  config: TokenPhysicsConfig,
): TokenRenderSnapshot {
  const x = Math.round((body.position.x - config.size / 2) * 10) / 10;
  const y = Math.round((body.position.y - config.size / 2) * 10) / 10;
  const angle = Math.round(body.angle * 1000) / 1000;
  return {
    opacity: "1",
    transform: `translate(${x}px, ${y}px) rotate(${angle}rad)`,
    // z-index 保持为正值，避免掉落过程中被容器背景遮住。
    zIndex: `${1000 + Math.max(0, Math.round(body.position.y))}`,
  };
}

function snapshotsMatch(
  left: TokenRenderSnapshot,
  right: TokenRenderSnapshot,
): boolean {
  return left.opacity === right.opacity
    && left.transform === right.transform
    && left.zIndex === right.zIndex;
}

function randomLauncherVelocity(min: number, max: number): number {
  const buffer = new Uint32Array(1);
  crypto.getRandomValues(buffer);
  return min + (buffer[0] / 0xffffffff) * (max - min);
}
