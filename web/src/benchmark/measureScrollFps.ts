export async function measureScrollFps(
  element: HTMLElement,
  durationMs: number,
): Promise<number | null> {
  const maximumScroll = element.scrollHeight - element.clientHeight;
  if (maximumScroll <= 0) {
    return null;
  }

  const initialScrollTop = element.scrollTop;
  const frameTimes: number[] = [];
  const start = performance.now();

  await new Promise<void>((resolve) => {
    const animate = (now: number) => {
      const progress = Math.min(1, (now - start) / durationMs);
      frameTimes.push(now);
      element.scrollTop = maximumScroll * progress;

      if (progress < 1) {
        requestAnimationFrame(animate);
        return;
      }
      resolve();
    };

    requestAnimationFrame(animate);
  });

  element.scrollTop = initialScrollTop;
  return calculateFps(frameTimes);
}

export function calculateFps(frameTimes: number[]): number {
  if (frameTimes.length < 2) {
    return 0;
  }
  const duration = frameTimes[frameTimes.length - 1] - frameTimes[0];
  return duration <= 0 ? 0 : ((frameTimes.length - 1) * 1000) / duration;
}
