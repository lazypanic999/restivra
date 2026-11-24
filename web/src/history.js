export const emptyHistoryFilters = {
  q: "",
  status: "",
  notification: "",
  from: "",
  to: "",
  project_id: "",
};

export function historyFiltersFromSearch(search) {
  const query = new URLSearchParams(search);
  return Object.fromEntries(
    Object.keys(emptyHistoryFilters).map((key) => [key, query.get(key) || ""]),
  );
}

export function initialPage(search) {
  const view = new URLSearchParams(search).get("view");
  return ["history", "projects", "setup"].includes(view) ? view : "overview";
}

function localDay(value, followingDay = false) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) throw Error("historyInvalidDates");
  const [year, month, day] = value.split("-").map(Number);
  const date = new Date(year, month - 1, day);
  if (
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day ||
    year < 1900 ||
    year > 9998
  )
    throw Error("historyInvalidDates");
  // Calendar arithmetic, not +24h: local days may span 23 or 25 hours at DST.
  if (followingDay) date.setDate(date.getDate() + 1);
  return date.toISOString();
}

export function historyParams(filters) {
  const query = new URLSearchParams();
  if (filters.q.trim()) query.set("q", filters.q.trim());
  if (filters.status) query.set("status", filters.status);
  if (filters.notification) query.set("notification", filters.notification);
  if (filters.project_id) query.set("project_id", filters.project_id);
  if (filters.from) query.set("since", localDay(filters.from));
  if (filters.to) query.set("until", localDay(filters.to, true));
  if (filters.from && filters.to && filters.from > filters.to)
    throw Error("historyInvalidDates");
  return query;
}

export function historySearch(filters, search = "") {
  const query = new URLSearchParams(search);
  query.set("view", "history");
  for (const key of Object.keys(emptyHistoryFilters)) {
    if (filters[key]) query.set(key, filters[key]);
    else query.delete(key);
  }
  return query.toString();
}
