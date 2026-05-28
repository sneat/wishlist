// SVG polyline `points` for a price-history series. X axis is date-based so
// missing days render as visual gaps rather than being compressed.
export function pricePolylinePoints(
  points: { date: string; price: number }[],
  width: number,
  height: number,
  yMax: number,
): string {
  if (points.length < 2) {
    return "";
  }

  const times = points.map((p) => new Date(p.date).getTime());
  const minTime = times[0];
  const timeRange = times[times.length - 1] - minTime || 1;

  return points
    .map((p, i) => {
      const x = ((times[i] - minTime) / timeRange) * width;
      const y = height - (p.price / yMax) * height;
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
}

// niceYMax picks a clean axis ceiling above maxPrice with breathing room (~10%).
// It walks step sizes (5, 10, 25, 50, ...) and returns the first one whose
// rounded-up ceiling falls within ~10 ticks, so the axis always has nice round
// labels at quartile divisions.
export function niceYMax(maxPrice: number): number {
  const target = maxPrice * 1.1;
  for (const step of [5, 10, 25, 50, 100, 250, 500]) {
    const ceil = Math.ceil(target / step) * step;
    if (ceil / step <= 10) return ceil;
  }
  return Math.ceil(target / 1000) * 1000;
}
