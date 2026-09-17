#!/usr/bin/env node
"use strict";
// Locate the platform-specific binary shipped by the matching optional
// dependency (@sightmap/jev-turbo-<os>-<arch>) and exec it. npm installs only
// the one package matching the host's os/cpu, so there is no download and no
// install script.

const path = require("node:path");
const { spawnSync } = require("node:child_process");

const key = `${process.platform}-${process.arch}`;
const pkgName = `@sightmap/jev-turbo-${key}`;
const binaryName = process.platform === "win32" ? "jev-turbo.exe" : "jev-turbo";

function resolveBinary() {
  try {
    const pkgJson = require.resolve(`${pkgName}/package.json`);
    return path.join(path.dirname(pkgJson), binaryName);
  } catch (_) {
    return null;
  }
}

const binary = resolveBinary();
if (!binary) {
  console.error(
    `jev-turbo: no prebuilt binary found for ${key}.\n` +
      `Expected the optional dependency ${pkgName}. This usually means either:\n` +
      `  - your platform/arch is unsupported — see https://github.com/sightmap/jev-turbo/releases\n` +
      `  - optional dependencies were skipped (e.g. --omit=optional). Try: npm install --force @sightmap/jev-turbo`
  );
  process.exit(1);
}

const result = spawnSync(binary, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error("jev-turbo:", result.error.code === "ENOENT" ? `binary not found at ${binary}` : result.error.message);
  process.exit(1);
}
process.exit(result.status ?? 1);
