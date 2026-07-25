import { spawn } from "node:child_process";
import { createRequire } from "node:module";

const [command = "dev", ...args] = process.argv.slice(2);
const port = process.env.WEB_PORT;
const require = createRequire(import.meta.url);
const nextBin = require.resolve("next/dist/bin/next");

if (!port) {
  console.error("WEB_PORT must be set");
  process.exit(1);
}

const child = spawn(
  process.execPath,
  [nextBin, command, "--port", port, ...args],
  { stdio: "inherit", env: process.env },
);

child.on("exit", (code, signal) => {
  if (signal) process.kill(process.pid, signal);
  else process.exit(code ?? 1);
});
