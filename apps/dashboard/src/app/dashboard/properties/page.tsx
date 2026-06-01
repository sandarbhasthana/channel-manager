import { listProperties } from "@/lib/api";
import { PropertiesGrid } from "@/components/properties/properties-grid";

export const metadata = { title: "Properties — Channel Manager" };
export const dynamic = "force-dynamic";

export default async function PropertiesPage() {
  const properties = await listProperties();

  return <PropertiesGrid properties={properties} />;
}
