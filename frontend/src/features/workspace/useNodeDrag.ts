import { useRef, useState, type HTMLAttributes } from "react";
import type { State } from "../../../bindings/agentor/internal/workspace/models";
import { locate, type NodeKind } from "./selection";
import { nodeMove } from "./reorder";

export function useNodeDrag(state: State | undefined, busy: boolean, onMove: (kind: NodeKind, id: string, offset: number) => void) {
  const [dragging, setDragging] = useState("");
  const [drop, setDrop] = useState<{ id: string; after: boolean }>();
  const suppressClick = useRef(false);
  const finish = () => {
    setDragging("");
    setDrop(undefined);
    setTimeout(() => { suppressClick.current = false; }, 0);
  };

  function row(id: string): HTMLAttributes<HTMLDivElement> {
    return {
      draggable: !busy,
      style: {
        opacity: dragging === id ? 0.5 : undefined,
        boxShadow: drop?.id === id ? `0 ${drop.after ? 2 : -2}px 0 0 var(--primary)` : undefined,
      },
      onDragStart: event => {
        if (busy) { event.preventDefault(); return; }
        event.stopPropagation();
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData("text/plain", id);
        suppressClick.current = true;
        setDragging(id);
      },
      onDragOver: event => {
        const box = event.currentTarget.getBoundingClientRect();
        const after = event.clientY >= box.top + box.height / 2;
        if (busy || !state || !dragging || !nodeMove(state, dragging, id, after)) {
          event.dataTransfer.dropEffect = "none";
          setDrop(undefined);
          return;
        }
        event.preventDefault();
        event.dataTransfer.dropEffect = "move";
        setDrop({ id, after });
      },
      onDragLeave: event => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDrop(current => current?.id === id ? undefined : current);
      },
      onDrop: event => {
        event.preventDefault();
        event.stopPropagation();
        if (!busy && state && dragging) {
          const box = event.currentTarget.getBoundingClientRect();
          const move = nodeMove(state, dragging, id, event.clientY >= box.top + box.height / 2);
          if (move) onMove(move.kind, move.id, move.offset);
        }
        finish();
      },
      onDragEnd: finish,
      onClickCapture: event => {
        if (suppressClick.current) { event.preventDefault(); event.stopPropagation(); }
      },
      onKeyDown: event => {
        if (busy || !state || !event.altKey || !["ArrowUp", "ArrowDown"].includes(event.key)) return;
        const node = locate(state, id);
        if (!node) return;
        event.preventDefault();
        event.stopPropagation();
        onMove(node.kind, id, event.key === "ArrowUp" ? -1 : 1);
      },
    };
  }

  const scroll: HTMLAttributes<HTMLDivElement>["onDragOver"] = event => {
    if (!dragging || busy) return;
    const box = event.currentTarget.getBoundingClientRect();
    if (event.clientY < box.top + 40) event.currentTarget.scrollBy(0, -20);
    else if (event.clientY > box.bottom - 40) event.currentTarget.scrollBy(0, 20);
    if (event.target === event.currentTarget) setDrop(undefined);
  };

  return { row, scroll };
}
