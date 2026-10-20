export type DiffRow = { kind: "header" | "added" | "removed" | "context"; oldLine: number | null; newLine: number | null; text: string };

export function parseDiff(diff: string, fileKind = "update"): DiffRow[] {
  if (fileKind === "add" || fileKind === "delete") return diff === "" ? [] : diff.replace(/\n$/, "").split("\n").map((text, index) => ({ kind: fileKind === "add" ? "added" : "removed", oldLine: fileKind === "delete" ? index + 1 : null, newLine: fileKind === "add" ? index + 1 : null, text }));
  let before = 0, after = 0, inHunk = false;
  return diff.replace(/\n$/, "").split("\n").map(line => {
    const hunk = line.match(/^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/);
    if (hunk) { before = Number(hunk[1]); after = Number(hunk[2]); inHunk = true; return { kind: "header", oldLine: null, newLine: null, text: line }; }
    if (line.startsWith("diff --git ")) { before = 0; after = 0; inHunk = false; }
    const header = line.startsWith("diff ") || line.startsWith("index ") || line.startsWith("\\") || (!inHunk && (line.startsWith("--- ") || line.startsWith("+++ ")));
    const added = !header && line.startsWith("+");
    const removed = !header && line.startsWith("-");
    return { kind: header ? "header" : added ? "added" : removed ? "removed" : "context",
      oldLine: !header && !added && before ? before++ : null,
      newLine: !header && !removed && after ? after++ : null,
      text: added || removed ? line.slice(1) : line };
  });
}
