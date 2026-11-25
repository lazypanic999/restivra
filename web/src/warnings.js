// Short, actionable labels for report warnings. The standing disclaimers
// that every report carries are left out.

const RULES = [
  [/^The backup is (\d+)% smaller than the median of the previous (\d+) runs/, (m) => ({ key: "warnShrank", severity: 3, values: { percent: m[1] } })],
  [/^The tested backup is identical to the previous run/, () => ({ key: "warnIdentical", severity: 2 })],
  [/^The backup file modification time is over 24 hours old/, () => ({ key: "warnOld", severity: 2 })],
  [/^No custom checks configured/, () => ({ key: "warnNoChecks", severity: 1 })],
];

export function attentionWarnings(report) {
  const found = [];
  for (const warning of report?.warnings || []) {
    for (const [pattern, make] of RULES) {
      const m = typeof warning === "string" && warning.match(pattern);
      if (m) {
        found.push(make(m));
        break;
      }
    }
  }
  return found.sort((a, b) => b.severity - a.severity);
}
