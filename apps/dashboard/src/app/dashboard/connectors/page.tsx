import { listConnections } from "@/lib/api";
import { ConnectorGrid } from "@/components/connectors/connector-grid";

export const metadata = { title: "OTA Connectors — Channel Manager" };

// Always fetch fresh — no caching, the page shows live connection state.
export const dynamic = "force-dynamic";

export default async function ConnectorsPage() {
  const connections = await listConnections();

  return <ConnectorGrid initialConnections={connections} />;
}
