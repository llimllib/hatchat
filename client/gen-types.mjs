#!/usr/bin/env node
// Generate TypeScript types from JSON Schema
// Usage: node gen-types.mjs (run from client/ directory)

import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { compile } from "json-schema-to-typescript";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");

const schemaPath = join(projectRoot, "schema", "protocol.json");
const outputPath = join(__dirname, "src", "protocol.generated.ts");

const schema = JSON.parse(readFileSync(schemaPath, "utf-8"));

const options = {
  bannerComment: "",
  additionalProperties: false,
  strictIndexSignatures: true,
  enableConstEnums: true,
  declareExternallyReferenced: true, // Include referenced types
};

/**
 * Resolve $refs in a schema by looking them up in the definitions
 */
function resolveRefs(obj, defs) {
  if (!obj || typeof obj !== "object") return obj;

  if (Array.isArray(obj)) {
    return obj.map((item) => resolveRefs(item, defs));
  }

  // If this is a $ref, resolve it
  if (obj.$ref) {
    const refPath = obj.$ref.replace("#/$defs/", "");
    const resolved = defs[refPath];
    if (resolved) {
      // Merge the resolved ref with any additional properties (like description)
      const { $ref: _ref, ...rest } = obj;
      return { ...resolveRefs(resolved, defs), ...rest };
    }
  }

  // Recursively resolve refs in nested objects
  const result = {};
  for (const [key, value] of Object.entries(obj)) {
    result[key] = resolveRefs(value, defs);
  }
  return result;
}

async function main() {
  try {
    const defs = schema.$defs || schema.definitions || {};
    const types = [];

    // Order matters - define base types first
    const typeOrder = [
      "User",
      "Room",
      "Message",
      "InitRequest",
      "SendMessageRequest",
      "HistoryRequest",
      "JoinRoomRequest",
      "InitResponse",
      "HistoryResponse",
      "JoinRoomResponse",
      "ErrorResponse",
      "Envelope",
    ];

    for (const name of typeOrder) {
      const defSchema = defs[name];
      if (!defSchema) {
        console.warn(`Warning: type ${name} not found in schema`);
        continue;
      }

      // Resolve any $refs in this schema
      const resolved = resolveRefs(defSchema, defs);

      // Create a standalone schema for this type
      const standaloneSchema = {
        $schema: schema.$schema,
        title: name,
        ...resolved,
      };

      const ts = await compile(standaloneSchema, name, options);

      // Post-process to fix biome complaints
      let processed = ts.trim();
      // Fix empty interfaces (biome prefers type aliases)
      processed = processed.replace(
        /export interface (\w+) \{\}/g,
        "export type $1 = Record<string, never>",
      );
      types.push(processed);
    }

    // Add header
    const header = `/* eslint-disable */
/**
 * This file was automatically generated from schema/protocol.json
 * DO NOT EDIT MANUALLY - run \`just client-types\` to regenerate
 */

`;

    // Add helper types at the end
    const helpers = `

// =============================================================================
// Helper types for working with the protocol
// =============================================================================

/**
 * All valid message type strings
 */
export type MessageType = "init" | "message" | "history" | "join_room" | "error";

/**
 * Type-safe envelope for client → server messages
 */
export type ClientEnvelope =
  | { type: "init"; data: InitRequest }
  | { type: "message"; data: SendMessageRequest }
  | { type: "history"; data: HistoryRequest }
  | { type: "join_room"; data: JoinRoomRequest };

/**
 * Type-safe envelope for server → client messages
 */
export type ServerEnvelope =
  | { type: "init"; data: InitResponse }
  | { type: "message"; data: Message }
  | { type: "history"; data: HistoryResponse }
  | { type: "join_room"; data: JoinRoomResponse }
  | { type: "error"; data: ErrorResponse };

/**
 * Type guard for checking message type
 */
export function isMessageType<T extends MessageType>(
  envelope: ServerEnvelope,
  type: T,
): envelope is Extract<ServerEnvelope, { type: T }> {
  return envelope.type === type;
}
`;

    writeFileSync(outputPath, header + types.join("\n\n") + helpers);
    console.log(`Generated ${outputPath}`);
  } catch (err) {
    console.error("Error generating types:", err);
    process.exit(1);
  }
}

main();
