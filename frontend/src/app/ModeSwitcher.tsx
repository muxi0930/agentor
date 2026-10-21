import { ChevronsUpDown, Layers3, MessageSquare } from "lucide-react";
import { DropdownMenu, DropdownMenuContent, DropdownMenuGroup, DropdownMenuLabel, DropdownMenuRadioGroup, DropdownMenuRadioItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem, useSidebar } from "@/components/ui/sidebar";

const modes = [
  { value: "chat", label: "会话", description: "所有项目与会话", icon: MessageSquare },
  { value: "workspace", label: "工作区", description: "项目、版本与流水线", icon: Layers3 },
];

export function ModeSwitcher({ mode, onChange }: { mode: string; onChange: (mode: string) => void }) {
  const { isMobile } = useSidebar();
  const current = modes.find(item => item.value === mode) || modes[0];
  const Icon = current.icon;
  return <SidebarMenu><SidebarMenuItem>
    <DropdownMenu>
      <DropdownMenuTrigger render={<SidebarMenuButton size="lg" tooltip={current.label} aria-label="切换会话或工作区" className="data-open:bg-sidebar-accent data-open:text-sidebar-accent-foreground" />}>
        <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground"><Icon className="size-4" /></span>
        <span className="grid min-w-0 flex-1 text-left leading-tight group-data-[collapsible=icon]:hidden">
          <span className="truncate text-sm font-medium">agentor · {current.label}</span>
          <span className="truncate text-xs text-muted-foreground">{current.description}</span>
        </span>
        <ChevronsUpDown className="ml-auto group-data-[collapsible=icon]:hidden" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent side={isMobile ? "bottom" : "right"} align="start" sideOffset={4} className="min-w-56">
        <DropdownMenuGroup>
          <DropdownMenuLabel>工作模式</DropdownMenuLabel>
          <DropdownMenuRadioGroup value={mode} onValueChange={value => onChange(String(value))}>
            {modes.map(item => <DropdownMenuRadioItem key={item.value} value={item.value} closeOnClick>
              <item.icon className="size-4" aria-hidden="true" />
              <span><span className="block">{item.label}</span><span className="block text-xs text-muted-foreground">{item.description}</span></span>
            </DropdownMenuRadioItem>)}
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  </SidebarMenuItem></SidebarMenu>;
}
