#!/usr/bin/env node
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { open, readdir, stat, truncate } from "node:fs/promises";
import { createReadStream } from "node:fs";
import { createInterface } from "node:readline";
import { basename, join } from "node:path";

const repoRoot = process.cwd();
const logDir = join(repoRoot, "log");
const repoName = basename(repoRoot);

async function listLogs() {
  const entries = await readdir(logDir);
  const logs = entries.filter((f) => f.endsWith(".log"));
  const withStats = await Promise.all(
    logs.map(async (f) => {
      const s = await stat(join(logDir, f));
      return {
        name: f.replace(/\.log$/, ""),
        filename: f,
        size: s.size,
        mtime: s.mtime.toISOString(),
      };
    }),
  );
  return withStats.sort((a, b) => b.mtime.localeCompare(a.mtime));
}

async function tailLog(name, lines) {
  const filename = name.endsWith(".log") ? name : `${name}.log`;
  const path = join(logDir, filename);
  const handle = await open(path, "r");
  try {
    const { size } = await handle.stat();
    const chunkSize = 64 * 1024;
    let pos = size;
    let buffer = Buffer.alloc(0);
    let newlineCount = 0;
    while (pos > 0 && newlineCount <= lines) {
      const readSize = Math.min(chunkSize, pos);
      pos -= readSize;
      const chunk = Buffer.alloc(readSize);
      await handle.read(chunk, 0, readSize, pos);
      buffer = Buffer.concat([chunk, buffer]);
      newlineCount = 0;
      for (const byte of buffer) if (byte === 0x0a) newlineCount++;
    }
    const text = buffer.toString("utf8");
    const split = text.split("\n");
    const tail = split.slice(-lines - 1);
    return { path, bytes: size, content: tail.join("\n") };
  } finally {
    await handle.close();
  }
}

function resolveLogPath(name) {
  const filename = name.endsWith(".log") ? name : `${name}.log`;
  return join(logDir, filename);
}

async function truncateLog(name) {
  const path = resolveLogPath(name);
  const before = (await stat(path)).size;
  await truncate(path, 0);
  return { path, bytesFreed: before };
}

async function searchLog(name, pattern, { maxMatches, ignoreCase, context }) {
  const path = resolveLogPath(name);
  const flags = ignoreCase ? "i" : "";
  let regex;
  try {
    regex = new RegExp(pattern, flags);
  } catch (e) {
    throw new Error(`Invalid regex: ${e.message}`);
  }
  const stream = createReadStream(path, { encoding: "utf8" });
  const rl = createInterface({ input: stream, crlfDelay: Infinity });
  const before = [];
  const matches = [];
  let lineNo = 0;
  let totalMatches = 0;
  let pendingAfter = 0;
  let currentMatch = null;

  for await (const line of rl) {
    lineNo++;
    if (pendingAfter > 0 && currentMatch) {
      currentMatch.after.push({ lineNo, line });
      pendingAfter--;
      if (pendingAfter === 0) currentMatch = null;
    }
    if (regex.test(line)) {
      totalMatches++;
      if (matches.length < maxMatches) {
        const entry = {
          lineNo,
          line,
          before: before.slice(),
          after: [],
        };
        matches.push(entry);
        currentMatch = entry;
        pendingAfter = context;
      }
    }
    before.push({ lineNo, line });
    if (before.length > context) before.shift();
  }
  rl.close();
  stream.destroy();
  return { path, totalMatches, shown: matches.length, matches };
}

function formatSearchResult({ path, totalMatches, shown, matches }, pattern, { ignoreCase, context }) {
  const header = `# ${path}\n# pattern: /${pattern}/${ignoreCase ? "i" : ""}  matches: ${totalMatches}${totalMatches > shown ? ` (showing first ${shown})` : ""}\n`;
  if (matches.length === 0) return header + "\n(no matches)";
  const blocks = matches.map((m) => {
    const lines = [];
    if (context > 0) for (const b of m.before) lines.push(`  ${b.lineNo}: ${b.line}`);
    lines.push(`> ${m.lineNo}: ${m.line}`);
    if (context > 0) for (const a of m.after) lines.push(`  ${a.lineNo}: ${a.line}`);
    return lines.join("\n");
  });
  return header + "\n" + blocks.join("\n---\n");
}

const server = new Server(
  { name: "ws-dev", version: "0.1.0" },
  { capabilities: { tools: {} } },
);

server.setRequestHandler(ListToolsRequestSchema, async () => ({
  tools: [
    {
      name: "list_logs",
      description: "log/*.log 一覧（name, size, mtime）。詳細は ws-dev skill。",
      inputSchema: {
        type: "object",
        properties: {},
        additionalProperties: false,
      },
    },
    {
      name: "tail_log",
      description: "log/<name>.log の末尾 N 行。",
      inputSchema: {
        type: "object",
        properties: {
          name: { type: "string" },
          lines: { type: "number", default: 100 },
        },
        required: ["name"],
        additionalProperties: false,
      },
    },
    {
      name: "truncate_log",
      description: "log/<name>.log を 0 バイトに in-place truncate。",
      inputSchema: {
        type: "object",
        properties: { name: { type: "string" } },
        required: ["name"],
        additionalProperties: false,
      },
    },
    {
      name: "search_log",
      description: "log/<name>.log を regex で検索（streaming）。",
      inputSchema: {
        type: "object",
        properties: {
          name: { type: "string" },
          pattern: { type: "string" },
          max_matches: { type: "number", default: 50 },
          context: { type: "number", default: 0 },
          ignore_case: { type: "boolean", default: false },
        },
        required: ["name", "pattern"],
        additionalProperties: false,
      },
    },
  ],
}));

server.setRequestHandler(CallToolRequestSchema, async (req) => {
  const { name, arguments: args = {} } = req.params;
  try {
    if (name === "list_logs") {
      const logs = await listLogs();
      return {
        content: [{ type: "text", text: JSON.stringify(logs, null, 2) }],
      };
    }
    if (name === "tail_log") {
      const logName = String(args.name ?? "");
      if (!logName) throw new Error("name is required");
      const lines = Number.isFinite(args.lines) ? Number(args.lines) : 100;
      const out = await tailLog(logName, lines);
      return {
        content: [
          {
            type: "text",
            text: `# ${out.path} (${out.bytes} bytes, last ${lines} lines)\n\n${out.content}`,
          },
        ],
      };
    }
    if (name === "truncate_log") {
      const logName = String(args.name ?? "");
      if (!logName) throw new Error("name is required");
      const out = await truncateLog(logName);
      return {
        content: [
          {
            type: "text",
            text: `Truncated ${out.path} (freed ${out.bytesFreed} bytes)`,
          },
        ],
      };
    }
    if (name === "search_log") {
      const logName = String(args.name ?? "");
      const pattern = String(args.pattern ?? "");
      if (!logName) throw new Error("name is required");
      if (!pattern) throw new Error("pattern is required");
      const opts = {
        maxMatches: Number.isFinite(args.max_matches) ? Number(args.max_matches) : 50,
        ignoreCase: Boolean(args.ignore_case),
        context: Number.isFinite(args.context) ? Number(args.context) : 0,
      };
      const result = await searchLog(logName, pattern, opts);
      return {
        content: [
          {
            type: "text",
            text: formatSearchResult(result, pattern, opts),
          },
        ],
      };
    }
    return {
      content: [{ type: "text", text: `Unknown tool: ${name}` }],
      isError: true,
    };
  } catch (e) {
    return {
      content: [{ type: "text", text: `Error: ${e.message}` }],
      isError: true,
    };
  }
});

await server.connect(new StdioServerTransport());
