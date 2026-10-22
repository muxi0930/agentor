import { CircleHelp, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Kbd, KbdGroup } from "@/components/ui/kbd";

const shortcuts = [
  { keys: [["Cmd", "="], ["Cmd", "+"]], description: "放大页面" },
  { keys: [["Cmd", "-"]], description: "缩小页面" },
  { keys: [["Cmd", "0"]], description: "恢复默认缩放" },
  { keys: [["Cmd", "B"]], description: "展开／收起侧栏" },
  { keys: [["Cmd", "["], ["Cmd", "]"]], description: "应用内后退／前进" },
  { keys: [["Enter"]], description: "聊天框发送消息；执行中追加指令" },
  { keys: [["Shift", "Enter"]], description: "聊天框换行" },
];

export function ShortcutHelp() {
  return <Dialog>
    <DialogTrigger render={<Button type="button" variant="ghost" size="sm" />}><CircleHelp aria-hidden="true" />帮助</DialogTrigger>
    <DialogContent showCloseButton={false} className="max-h-[calc(100dvh-2rem)] overflow-y-auto px-6 pt-12 pb-5 sm:max-w-xl">
      <DialogTitle className="sr-only">快捷键帮助</DialogTitle>
      <DialogDescription className="sr-only">应用支持的七组常用快捷键</DialogDescription>
      <DialogClose render={<Button type="button" variant="ghost" size="icon-sm" className="absolute top-3 right-3" aria-label="关闭帮助" />}><X aria-hidden="true" /></DialogClose>
      <dl className="divide-y divide-border">
        {shortcuts.map(shortcut => <div key={shortcut.description} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3 first:pt-0 last:pb-0">
          <dt className="text-sm">{shortcut.description}</dt>
          <dd className="flex shrink-0 items-center gap-2">
            {shortcut.keys.map((keys, index) => <span key={keys.join("+")} className="inline-flex items-center gap-2">
              {index > 0 && <span className="text-xs text-muted-foreground">／</span>}
              <KbdGroup>{keys.map(key => <Kbd key={key}>{key}</Kbd>)}</KbdGroup>
            </span>)}
          </dd>
        </div>)}
      </dl>
    </DialogContent>
  </Dialog>;
}
