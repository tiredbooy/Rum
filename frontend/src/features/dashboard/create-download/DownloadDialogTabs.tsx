import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { cn } from "@/lib/utils";
import { Download, FileText, Layers, Link } from "lucide-react";
import { useEffect, useState } from "react";
import { SingleDownloadForm } from "./SingleDownloadForm";
import { BulkDownloadForm } from "./BulkDownloadForm";
import { LoadFromUrlForm } from "./LoadFromUrlForm";
import { BatchDownloadForm } from "./BatchDownloadForm";
import { useAddDialogStore } from "@/stores/add-dialog-store";

export function DownloadTabs() {
  const initialTab = useAddDialogStore((s) => s.initialTab);
  const [activeTab, setActiveTab] = useState<string>(initialTab);
  useEffect(() => setActiveTab(initialTab), [initialTab]);

  return (
    <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
      <TabsList className="grid w-full grid-cols-4 bg-muted rounded-md">
        <TabsTrigger
          value="single"
          className={cn(
            "flex items-center gap-2",
            "data-[state=active]:bg-background data-[state=active]:text-foreground",
            "text-muted-foreground",
          )}
        >
          <Download className="h-4 w-4" />
          <span className="hidden sm:inline">Single</span>
        </TabsTrigger>
        <TabsTrigger
          value="bulk"
          className={cn(
            "flex items-center gap-2",
            "data-[state=active]:bg-background data-[state=active]:text-foreground",
            "text-muted-foreground",
          )}
        >
          <FileText className="h-4 w-4" />
          <span className="hidden sm:inline">Bulk</span>
        </TabsTrigger>
        <TabsTrigger
          value="url"
          className={cn(
            "flex items-center gap-2",
            "data-[state=active]:bg-background data-[state=active]:text-foreground",
            "text-muted-foreground",
          )}
        >
          <Link className="h-4 w-4" />
          <span className="hidden sm:inline">From URL</span>
        </TabsTrigger>
        <TabsTrigger
          value="batch"
          className={cn(
            "flex items-center gap-2",
            "data-[state=active]:bg-background data-[state=active]:text-foreground",
            "text-muted-foreground",
          )}
        >
          <Layers className="h-4 w-4" />
          <span className="hidden sm:inline">Batch</span>
        </TabsTrigger>
      </TabsList>

      <TabsContent value="single" className="mt-4">
        <SingleDownloadForm />
      </TabsContent>

      <TabsContent value="bulk" className="mt-4">
        <BulkDownloadForm />
      </TabsContent>

      <TabsContent value="url" className="mt-4">
        <LoadFromUrlForm />
      </TabsContent>

      <TabsContent value="batch" className="mt-4">
        <BatchDownloadForm />
      </TabsContent>
    </Tabs>
  );
}
