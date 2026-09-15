export type VirtualRange = {
  startIndex: number;
  endIndex: number;
  totalHeight: number;
};

type VirtualRangeOptions = {
  itemCount: number;
  rowHeight: number;
  viewportHeight: number;
  scrollTop: number;
  overscan: number;
};

// Calcule les indices à afficher sans créer les 100 000 lignes dans le DOM.
export function getVirtualRange(options: VirtualRangeOptions): VirtualRange {
  const { itemCount, rowHeight, viewportHeight, scrollTop, overscan } = options;
  if (itemCount <= 0) {
    return { startIndex: 0, endIndex: 0, totalHeight: 0 };
  }

  const firstVisible = Math.min(itemCount - 1, Math.floor(scrollTop / rowHeight));
  const startIndex = Math.max(0, firstVisible - overscan);
  const visibleCount = Math.ceil(viewportHeight / rowHeight) + overscan * 2;
  const endIndex = Math.min(itemCount, startIndex + visibleCount);

  return { startIndex, endIndex, totalHeight: itemCount * rowHeight };
}
