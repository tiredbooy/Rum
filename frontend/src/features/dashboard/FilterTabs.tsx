import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { ReactNode } from "react";

export interface TabItem {
  value: string;
  label: string;
  content: ReactNode; // the component you want to render
}

interface FilterTabsProps {
  tabs: TabItem[];
  defaultValue?: string;
  onTabChange?: (value: string) => void;
  className?: string;
}

export function FilterTabs({
  tabs,
  defaultValue,
  onTabChange,
  className,
}: FilterTabsProps) {
  return (
    <Tabs
      defaultValue={defaultValue ?? tabs[0]?.value}
      onValueChange={onTabChange}
      className={className}
    >
      <TabsList className="w-fit">
        {tabs.map((tab) => (
          <TabsTrigger key={tab.value} value={tab.value}>
            {tab.label}
          </TabsTrigger>
        ))}
      </TabsList>

      {tabs.map((tab) => (
        <TabsContent key={tab.value} value={tab.value} className="mt-4">
          {tab.content}
        </TabsContent>
      ))}
    </Tabs>
  );
}
