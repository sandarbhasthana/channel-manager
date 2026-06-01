"use client";

import { useState } from "react";
import { createKeyAction } from "./actions";
import { Plus, X } from "lucide-react";

export function CreateKeyModal() {
  const [isOpen, setIsOpen] = useState(false);
  const [isPending, setIsPending] = useState(false);
  const [error, setError] = useState("");
  const [secret, setSecret] = useState("");
  const [copied, setCopied] = useState(false);

  const openModal = () => setIsOpen(true);
  const closeModal = () => {
    setIsOpen(false);
    setError("");
    setSecret("");
    setCopied(false);
  };

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setIsPending(true);
    setError("");
    setSecret("");
    setCopied(false);
    
    const formData = new FormData(e.currentTarget);
    const result = await createKeyAction(formData);
    
    if (result.error) {
      setError(result.error);
    } else if (result.secretKey) {
      setSecret(result.secretKey);
    }
    
    setIsPending(false);
  }

  const handleCopy = () => {
    navigator.clipboard.writeText(secret);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <>
      <button
        onClick={openModal}
        className="inline-flex items-center gap-1.5 px-3 py-1.5 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-md transition-colors"
      >
        <Plus size={16} />
        Create new secret key
      </button>

      {isOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
          <div className="bg-white rounded-lg shadow-xl w-full max-w-xl overflow-hidden animate-in fade-in zoom-in-95 duration-200">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-100">
              <h2 className="text-lg font-semibold text-gray-900">
                {secret ? "Save your secret key" : "Create new secret key"}
              </h2>
              <button
                onClick={closeModal}
                className="text-gray-400 hover:text-gray-600 transition-colors"
              >
                <X size={20} />
              </button>
            </div>

            <div className="px-6 py-5">
              {secret ? (
                <div className="space-y-4">
                  <div className="bg-green-50 border border-green-200 rounded-md p-4">
                    <p className="text-sm text-green-800">
                      Please save this secret key somewhere safe and accessible. For security reasons, <strong>you won't be able to view it again</strong> through your account. If you lose this secret key, you'll need to generate a new one.
                    </p>
                  </div>
                  <div className="mt-3">
                    <div className="flex items-center border border-gray-200 rounded bg-gray-50 p-1">
                      <code className="flex-1 px-3 py-2 text-sm text-gray-900 whitespace-nowrap overflow-x-auto select-all font-mono">
                        {secret}
                      </code>
                      <button
                        type="button"
                        onClick={handleCopy}
                        className="ml-2 px-3 py-1.5 text-xs font-medium text-gray-600 bg-white border border-gray-200 rounded hover:bg-gray-50 transition-colors shrink-0"
                      >
                        {copied ? "Copied!" : "Copy"}
                      </button>
                    </div>
                  </div>
                  <div className="mt-6 flex justify-end">
                    <button
                      type="button"
                      onClick={closeModal}
                      className="px-4 py-2 text-sm font-medium text-white bg-black hover:bg-gray-800 rounded-md transition-colors"
                    >
                      Done
                    </button>
                  </div>
                </div>
              ) : (
                <form onSubmit={handleSubmit} className="space-y-5">
                  {error && (
                    <div className="bg-red-50 text-red-600 text-sm p-3 rounded-md border border-red-100">
                      {error}
                    </div>
                  )}
                  
                  <div>
                    <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-1.5">
                      Name
                    </label>
                    <input
                      type="text"
                      name="name"
                      id="name"
                      required
                      placeholder="e.g. MyPMS Production"
                      className="block w-full text-sm border border-gray-300 rounded-md px-3 py-2 focus:ring-1 focus:ring-black focus:border-black outline-none transition-shadow"
                    />
                  </div>
                  
                  <div className="pt-2 flex justify-end gap-3">
                    <button
                      type="button"
                      onClick={closeModal}
                      className="px-4 py-2 text-sm font-medium text-gray-700 bg-white border border-gray-300 rounded-md hover:bg-gray-50 transition-colors"
                    >
                      Cancel
                    </button>
                    <button
                      type="submit"
                      disabled={isPending}
                      className="px-4 py-2 text-sm font-medium text-white bg-green-600 hover:bg-green-700 rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed inline-flex items-center"
                    >
                      {isPending ? "Creating..." : "Create secret key"}
                    </button>
                  </div>
                </form>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  );
}
