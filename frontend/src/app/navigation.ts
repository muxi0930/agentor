export type Destination = { page: string; mode: string };
export type NavigationHistory = { entries: Destination[]; index: number };

export function visit(history: NavigationHistory, destination: Destination): NavigationHistory {
  const current = history.entries[history.index];
  if (current?.page === destination.page && current.mode === destination.mode) return history;
  const entries = [...history.entries.slice(0, history.index + 1), destination];
  return { entries, index: entries.length - 1 };
}

export function move(history: NavigationHistory, offset: number): NavigationHistory {
  const index = history.index + offset;
  return index < 0 || index >= history.entries.length ? history : { ...history, index };
}
