export interface HomeAsciiParticle {
  alpha: number;
  char: string;
  delay: number;
  isText: boolean;
  phase: number;
  targetAlpha: number;
  tx: number;
  ty: number;
  vx: number;
  vy: number;
  x: number;
  y: number;
}

export interface HomeAsciiPointer {
  x: number;
  y: number;
}

interface CreateParticleFieldOptions {
  charset: string;
  height: number;
  imageData: ImageData;
  isMobile: boolean;
  step: number;
  width: number;
}

interface UpdateParticleOptions {
  charset: string;
  elapsed: number;
  height: number;
  influenceForce: number;
  influenceRadius: number;
  motionScale: number;
  pointer: HomeAsciiPointer | null;
  width: number;
}

export function createHomeAsciiParticleField({
  charset,
  height,
  imageData,
  isMobile,
  step,
  width,
}: CreateParticleFieldOptions): HomeAsciiParticle[] {
  const particles = createTextParticles({
    charset,
    height,
    imageData,
    isMobile,
    step,
    width,
  });
  const ambientCount = Math.max(40, Math.floor(particles.length * 0.12));
  for (let index = 0; index < ambientCount; index += 1) {
    particles.push(createAmbientParticle(charset, width, height));
  }
  return particles;
}

export function updateHomeAsciiParticle(
  particle: HomeAsciiParticle,
  options: UpdateParticleOptions,
): number {
  const progress = Math.max(0, options.elapsed - particle.delay);
  if (particle.isText && progress < 0.01) {
    return 0.02;
  }

  const stepCount = Math.max(1, Math.ceil(options.motionScale));
  const stepScale = options.motionScale / stepCount;
  for (let step = 0; step < stepCount; step += 1) {
    particle.vx += (particle.tx - particle.x) * 0.038 * stepScale;
    particle.vy += (particle.ty - particle.y) * 0.038 * stepScale;
    applyPointerForce(particle, options, stepScale);
    const damping = 0.87 ** stepScale;
    particle.vx *= damping;
    particle.vy *= damping;
    particle.x += particle.vx * stepScale;
    particle.y += particle.vy * stepScale;
    particle.alpha += (particle.targetAlpha - particle.alpha)
      * (1 - 0.96 ** stepScale);
  }

  if (particle.isText) {
    updateTextParticle(particle, options.charset, options.elapsed, progress);
  } else {
    updateAmbientParticle(
      particle,
      options.charset,
      options.width,
      options.height,
      options.motionScale,
    );
  }
  return Math.max(0, particle.alpha);
}

function createTextParticles({
  charset,
  height,
  imageData,
  isMobile,
  step,
  width,
}: CreateParticleFieldOptions): HomeAsciiParticle[] {
  const particles: HomeAsciiParticle[] = [];
  for (let y = 0; y < height; y += step) {
    for (let x = 0; x < width; x += step) {
      if (imageData.data[(y * width + x) * 4 + 3] <= 80) {
        continue;
      }
      particles.push({
        alpha: 0,
        char: pickCharacter(charset),
        delay: x / width + Math.random() * 0.15,
        isText: true,
        phase: Math.random() * Math.PI * 2,
        targetAlpha: isMobile ? 0.95 : 0.82 + Math.random() * 0.18,
        tx: x,
        ty: y,
        vx: 0,
        vy: 0,
        x: x + (Math.random() - 0.5) * width * 0.45,
        y: y + (Math.random() - 0.5) * height * 2.2,
      });
    }
  }
  return particles;
}

function createAmbientParticle(
  charset: string,
  width: number,
  height: number,
): HomeAsciiParticle {
  const x = Math.random() * width;
  const y = Math.random() * height;
  return {
    alpha: 0,
    char: pickCharacter(charset),
    delay: Math.random() * 0.5,
    isText: false,
    phase: Math.random() * Math.PI * 2,
    targetAlpha: 0.03 + Math.random() * 0.06,
    tx: x,
    ty: y,
    vx: (Math.random() - 0.5) * 0.12,
    vy: (Math.random() - 0.5) * 0.12,
    x,
    y,
  };
}

function applyPointerForce(
  particle: HomeAsciiParticle,
  { influenceForce, influenceRadius, pointer }: UpdateParticleOptions,
  motionScale: number,
): void {
  if (!pointer) {
    return;
  }
  const dx = particle.x - pointer.x;
  const dy = particle.y - pointer.y;
  const distanceSq = dx * dx + dy * dy;
  if (distanceSq <= 0 || distanceSq >= influenceRadius * influenceRadius) {
    return;
  }
  const distance = Math.sqrt(distanceSq);
  const force = ((1 - distance / influenceRadius) ** 2)
    * influenceForce
    * motionScale;
  particle.vx += (dx / distance) * force;
  particle.vy += (dy / distance) * force;
}

function updateTextParticle(
  particle: HomeAsciiParticle,
  charset: string,
  elapsed: number,
  progress: number,
): void {
  particle.alpha = particle.targetAlpha
    + Math.sin(elapsed * 0.7 + particle.phase) * 0.07;
  if (progress < 0.9 || Math.random() < 0.0006) {
    particle.char = pickCharacter(charset);
  }
}

function updateAmbientParticle(
  particle: HomeAsciiParticle,
  charset: string,
  width: number,
  height: number,
  motionScale: number,
): void {
  const driftScale = Math.sqrt(motionScale);
  particle.tx += (Math.random() - 0.5) * 0.18 * driftScale;
  particle.ty += (Math.random() - 0.5) * 0.18 * driftScale;
  [particle.x, particle.tx] = wrapCoordinate(particle.x, particle.tx, width);
  [particle.y, particle.ty] = wrapCoordinate(particle.y, particle.ty, height);
  if (Math.random() < scaleFrameProbability(0.003, motionScale)) {
    particle.char = pickCharacter(charset);
  }
}

function scaleFrameProbability(probability: number, motionScale: number): number {
  return 1 - (1 - probability) ** motionScale;
}

function wrapCoordinate(
  value: number,
  target: number,
  boundary: number,
): [number, number] {
  if (value < -20) {
    return [boundary + 10, boundary + 10];
  }
  if (value > boundary + 20) {
    return [-10, -10];
  }
  return [value, target];
}

function pickCharacter(charset: string): string {
  return charset[Math.floor(Math.random() * charset.length)] ?? ".";
}
