const fs = require("fs");
const path = require("path");

const filePath = path.join(__dirname, "CalendarRatePriceModal.tsx");
let content = fs.readFileSync(filePath, "utf-8");

// Remove shadcn imports
content = content.replace(/import { Button } from "@\/components\/ui\/button";\n/g, "");
content = content.replace(/import { Input } from "@\/components\/ui\/input";\n/g, "");
content = content.replace(/import { Label } from "@\/components\/ui\/label";\n/g, "");
content = content.replace(/import { Checkbox } from "@\/components\/ui\/checkbox";\n/g, "");
content = content.replace(/import \{\n  Dialog,\n  DialogContent,\n  DialogDescription,\n  DialogFooter,\n  DialogHeader,\n  DialogTitle\n\} from "@\/components\/ui\/dialog";\n/g, "");
content = content.replace(/import \{\n  Select,\n  SelectContent,\n  SelectItem,\n  SelectTrigger,\n  SelectValue\n\} from "@\/components\/ui\/select";\n/g, "");
content = content.replace(/import \{\n  Tooltip,\n  TooltipContent,\n  TooltipProvider,\n  TooltipTrigger\n\} from "@\/components\/ui\/tooltip";\n/g, "");

// Add antd imports
content = content.replace(/import { IndianRupee, X } from "lucide-react";\n/, `import { IndianRupee, X } from "lucide-react";\nimport { Modal, Button, Input, Checkbox, Select, Tooltip } from "antd";\n`);

// Replace TooltipProvider and Dialog wrappers with Modal
content = content.replace(/<TooltipProvider>[\s\S]*?<Dialog open=\{open\} onOpenChange=\{\(o\) => !o && onClose\(\)\}>[\s\S]*?<DialogContent[^>]*>([\s\S]*?)<\/DialogContent>[\s\S]*?<\/Dialog>[\s\S]*?<\/TooltipProvider>/, function(match, innerContent) {
  // Extract Header, Footer, and body
  const headerMatch = innerContent.match(/<DialogHeader>([\s\S]*?)<\/DialogHeader>/);
  const footerMatch = innerContent.match(/<DialogFooter[^>]*>([\s\S]*?)<\/DialogFooter>/);
  let body = innerContent;
  if (headerMatch) body = body.replace(headerMatch[0], "");
  if (footerMatch) body = body.replace(footerMatch[0], "");
  
  let headerCode = headerMatch ? headerMatch[1].replace(/<DialogTitle[^>]*>([\s\S]*?)<\/DialogTitle>/g, "<div className=\"text-lg font-semibold\">$1</div>").replace(/<DialogDescription[^>]*>([\s\S]*?)<\/DialogDescription>/g, "<div className=\"text-sm text-gray-500 mt-1\">$1</div>") : "";
  let footerCode = footerMatch ? footerMatch[1] : "";

  return `<Modal
      open={open}
      onCancel={onClose}
      footer={<div className="flex justify-end gap-2 mt-4">${footerCode}</div>}
      title={<div>${headerCode}</div>}
      width={800}
      centered
      closeIcon={<X className="h-4 w-4" />}
    >
      ${body}
    </Modal>`;
});

// Replace Selects
content = content.replace(/<Select\s+value=\{updateType\}\s+onValueChange=\{\(v\) => \{\s+setUpdateType\(v as "set" \| "adjust"\);\s+setPriceValue\(v === "set" \? String\(currentPrice\) : ""\);\s+\}\}\s*>[\s\S]*?<\/Select>/g, `<Select value={updateType} onChange={(v) => { setUpdateType(v as "set" | "adjust"); setPriceValue(v === "set" ? String(currentPrice) : ""); }} className="w-full">
  <Select.Option value="set">Set Fixed Price</Select.Option>
  <Select.Option value="adjust">Adjust Existing Prices</Select.Option>
</Select>`);

content = content.replace(/<Select\s+value=\{adjustmentType\}\s+onValueChange=\{\(v\) => setAdjustmentType\(v as "amount" \| "percentage"\)\}\s*>[\s\S]*?<\/Select>/g, `<Select value={adjustmentType} onChange={(v) => setAdjustmentType(v as "amount" | "percentage")} className="w-full">
  <Select.Option value="amount">Amount (₹)</Select.Option>
  <Select.Option value="percentage">Percentage (%)</Select.Option>
</Select>`);

// Replace Label with simple <label>
content = content.replace(/<Label([^>]*)>/g, '<label$1>');
content = content.replace(/<\/Label>/g, '</label>');

// Replace Checkbox attributes: onCheckedChange -> onChange, checked -> checked
content = content.replace(/onCheckedChange=\{\(\) => toggleRoomType\(rt.roomTypeId\)\}/g, `onChange={() => toggleRoomType(rt.roomTypeId)}`);
content = content.replace(/onCheckedChange=\{\(c\) => handleWeekdaysToggle\(\!\!c\)\}/g, `onChange={(e) => handleWeekdaysToggle(e.target.checked)}`);
content = content.replace(/onCheckedChange=\{\(c\) => handleWeekendsToggle\(\!\!c\)\}/g, `onChange={(e) => handleWeekendsToggle(e.target.checked)}`);
content = content.replace(/onCheckedChange=\{\(\) => toggleDay\(i\)\}/g, `onChange={() => toggleDay(i)}`);
content = content.replace(/onCheckedChange=\{\(c\) => setRestrictions\(\(p\) => \(\{ \.\.\.p, closedToArrival: \!\!c \}\)\)\}/g, `onChange={(e) => setRestrictions((p) => ({ ...p, closedToArrival: e.target.checked }))}`);
content = content.replace(/onCheckedChange=\{\(c\) => setRestrictions\(\(p\) => \(\{ \.\.\.p, closedToDeparture: \!\!c \}\)\)\}/g, `onChange={(e) => setRestrictions((p) => ({ ...p, closedToDeparture: e.target.checked }))}`);

// Close tooltips properly
content = content.replace(/<Tooltip>[\s\S]*?<TooltipTrigger asChild>([\s\S]*?)<\/TooltipTrigger>[\s\S]*?<TooltipContent[^>]*><p>(.*?)<\/p><\/TooltipContent>[\s\S]*?<\/Tooltip>/g, `<Tooltip title="$2">$1</Tooltip>`);

fs.writeFileSync(filePath, content);
console.log("Converted successfully.");
