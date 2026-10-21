import { useRef, type ComponentProps, type MouseEvent, type ReactNode } from "react";
import { cn } from "cn";
import { Button } from "@/components/ui/button";
import { SidebarMenuButton } from "@/components/ui/sidebar";
import "./sidebar-selection.css";

type SidebarSelectionButtonProps = Omit<ComponentProps<"button">, "children" | "onClickCapture"> & Pick<ComponentProps<typeof Button>, "variant" | "size"> & {
  active: boolean;
  sidebar?: boolean;
  tooltip?: string;
  children: ReactNode;
};

export function SidebarSelectionButton({active,sidebar=false,tooltip,children,className,variant="ghost",size="default",...props}:SidebarSelectionButtonProps){
  const fill=useRef<HTMLSpanElement>(null);
  const capture=(event:MouseEvent<HTMLButtonElement>)=>{
    const box=event.currentTarget.getBoundingClientRect();
    fill.current?.style.setProperty("--selection-x",event.detail===0?"50%":event.clientX-box.left+"px");
    fill.current?.style.setProperty("--selection-y",event.detail===0?"50%":event.clientY-box.top+"px");
  };
  const content=<>
    <span className="sidebar-selection-content">{children}</span>
    <span ref={fill} aria-hidden="true" data-active={active||undefined} className="sidebar-selection-fill">{children}</span>
  </>;
  const buttonClass=cn("sidebar-selection-button",sidebar?"sidebar-selection-strong":"sidebar-selection-subtle",className);
  return sidebar
    ?<SidebarMenuButton {...props} isActive={active} tooltip={tooltip} className={buttonClass} onClickCapture={capture}>{content}</SidebarMenuButton>
    :<Button {...props} variant={variant} size={size} data-active={active||undefined} className={buttonClass} onClickCapture={capture}>{content}</Button>;
}
