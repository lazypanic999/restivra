// Per-project trend summary built from retained history items.
// Items arrive newest first, as /api/history returns them.

export const TREND_WINDOW = 30;
const FINISHED = new Set(["passed", "failed", "error"]);

export function median(values) {
  if (!values.length) return null;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
}

const positive = (value) => typeof value === "number" && Number.isFinite(value) && value > 0;

// Compare the newest value with the median of up to ten earlier ones.
// Fewer than three earlier values is not a baseline.
function againstBaseline(values) {
  if (values.length < 4) return null;
  const latest = values[values.length - 1];
  const earlier = values.slice(-11, -1);
  const base = median(earlier);
  if (!positive(base)) return null;
  return { latest, base, ratio: latest / base, sample: earlier.length };
}

export function summarize(items) {
  const runs = items.filter((item) => FINISHED.has(item.status)).reverse();
  const passed = runs.filter((run) => run.status === "passed").length;
  const sizes = runs.filter((run) => positive(run.backup_bytes)).map((run) => run.backup_bytes);
  const durations = runs.filter((run) => positive(run.duration_ms)).map((run) => run.duration_ms);
  const signals = [];

  let failing = 0;
  for (let i = runs.length - 1; i >= 0 && runs[i].status !== "passed"; i--) failing++;
  if (failing >= 2) signals.push({ kind: "failingStreak", severity: 3, count: failing });

  const size = againstBaseline(sizes);
  if (size && size.ratio <= 0.5)
    signals.push({ kind: "archiveShrank", severity: 3, percent: Math.round((1 - size.ratio) * 100), sample: size.sample });
  else if (size && size.ratio >= 2)
    signals.push({ kind: "archiveGrew", severity: 1, percent: Math.round((size.ratio - 1) * 100), sample: size.sample });

  const hashed = runs.filter((run) => run.backup_sha256);
  let same = 1;
  for (let i = hashed.length - 1; i > 0 && hashed[i].backup_sha256 === hashed[i - 1].backup_sha256; i--) same++;
  if (hashed.length && same >= 3) signals.push({ kind: "archiveUnchanged", severity: 2, count: same });

  const duration = againstBaseline(durations);
  if (duration && duration.ratio >= 3)
    signals.push({ kind: "restoreSlower", severity: 1, times: Math.round(duration.ratio * 10) / 10, sample: duration.sample });

  signals.sort((a, b) => b.severity - a.severity);
  return {
    runs,
    passRate: runs.length ? passed / runs.length : null,
    sizes,
    durations,
    latestBytes: sizes.length ? sizes[sizes.length - 1] : null,
    medianDurationMS: median(durations),
    signals,
  };
}

// Polyline points for a sparkline inside a width x height box.
export function sparkPoints(values, width, height, pad = 2) {
  if (!values.length) return [];
  const min = Math.min(...values), max = Math.max(...values);
  const span = max - min || 1;
  const step = values.length > 1 ? (width - 2 * pad) / (values.length - 1) : 0;
  return values.map((value, i) => [
    +(pad + i * step).toFixed(1),
    +(max === min ? height / 2 : height - pad - ((value - min) / span) * (height - 2 * pad)).toFixed(1),
  ]);
}

// Run fn over items with at most `limit` in flight.
export async function mapLimited(items, limit, fn) {
  const results = new Array(items.length);
  let next = 0;
  const worker = async () => {
    while (next < items.length) {
      const i = next++;
      results[i] = await fn(items[i], i);
    }
  };
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker));
  return results;
}
