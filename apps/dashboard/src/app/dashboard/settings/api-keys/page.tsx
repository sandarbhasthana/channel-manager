import { listIntegrationKeys } from "@/lib/api";
import { revokeKeyAction } from "./actions";
import { CreateKeyModal } from "./create-key-modal";
import { format } from "date-fns";
import { Trash2 } from "lucide-react";

export const metadata = {
  title: "API Keys - Channel Manager",
};

export default async function ApiKeysPage() {
  const keys = await listIntegrationKeys();

  return (
    <div className="w-full px-8 py-8">
      <div className="mb-8">
        <h1 className="text-2xl font-semibold tracking-tight text-gray-900">
          API keys
        </h1>
        <p className="mt-2 text-sm text-gray-600">
          Your secret API keys are listed below. Please note that we do not display your secret API keys again after you generate them.
        </p>
        <p className="mt-2 text-sm text-gray-600">
          Do not share your API key with others, or expose it in the browser or other client-side code.
        </p>
      </div>

      <div className="bg-white rounded-lg border border-gray-200 overflow-hidden shadow-sm">
        <div className="px-6 py-4 border-b border-gray-200">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold text-gray-900">Keys</h2>
            <CreateKeyModal />
          </div>
        </div>
        
        {keys.length === 0 ? (
          <div className="px-6 py-12 text-center text-sm text-gray-500">
            No API keys found. Create one to connect your PMS.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full divide-y divide-gray-200 text-sm">
              <thead className="bg-gray-50/50">
                <tr>
                  <th scope="col" className="px-6 py-3 text-left font-medium text-gray-500 uppercase tracking-wider text-xs">
                    Name
                  </th>
                  <th scope="col" className="px-6 py-3 text-left font-medium text-gray-500 uppercase tracking-wider text-xs">
                    Secret Key
                  </th>
                  <th scope="col" className="px-6 py-3 text-left font-medium text-gray-500 uppercase tracking-wider text-xs">
                    Created
                  </th>
                  <th scope="col" className="px-6 py-3 text-left font-medium text-gray-500 uppercase tracking-wider text-xs">
                    Last Used
                  </th>
                  <th scope="col" className="relative px-6 py-3">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {keys.map((key) => (
                  <tr key={key.id} className="hover:bg-gray-50 transition-colors group">
                    <td className="px-6 py-4 whitespace-nowrap text-gray-900 font-medium">
                      {key.name}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-gray-600 font-mono text-xs">
                      {key.key_prefix}...
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-gray-500">
                      {format(new Date(key.created_at), "dd MMM yyyy")}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-gray-500">
                      Never
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                      <form action={async () => {
                        "use server";
                        await revokeKeyAction(key.id);
                      }}>
                        <button
                          type="submit"
                          className="text-gray-400 hover:text-red-600 transition-colors opacity-0 group-hover:opacity-100 focus:opacity-100"
                          title="Revoke key"
                        >
                          <Trash2 size={16} />
                        </button>
                      </form>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
