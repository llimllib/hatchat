#!/usr/bin/env node
// Generate Zod schemas and TypeScript types from JSON Schema
// Usage: node gen-types.mjs (run from client/ directory)

import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");

const schemaPath = join(projectRoot, "schema", "protocol.json");
const outputPath = join(__dirname, "src", "protocol.generated.ts");

const schema = JSON.parse(readFileSync(schemaPath, "utf-8"));

// Order matters - define base types first (dependencies before dependents)
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

/**
 * Convert a JSON Schema type definition to a Zod schema string
 */
function jsonSchemaToZod(schema, defs, depth = 0) {
  if (!schema) return "z.unknown()";

  // Handle $ref
  if (schema.$ref) {
    const refName = schema.$ref.replace("#/$defs/", "");
    return `${refName}Schema`;
  }

  const type = schema.type;

  if (type === "string") {
    let result = "z.string()";
    if (schema.pattern) {
      result += `.regex(/${schema.pattern}/)`;
    }
    if (schema.minLength !== undefined) {
      result += `.min(${schema.minLength})`;
    }
    if (schema.maxLength !== undefined) {
      result += `.max(${schema.maxLength})`;
    }
    return result;
  }

  if (type === "integer" || type === "number") {
    let result = type === "integer" ? "z.int()" : "z.number()";
    if (schema.minimum !== undefined) {
      result += `.min(${schema.minimum})`;
    }
    if (schema.maximum !== undefined) {
      result += `.max(${schema.maximum})`;
    }
    return result;
  }

  if (type === "boolean") {
    return "z.boolean()";
  }

  if (type === "array") {
    const items = jsonSchemaToZod(schema.items, defs, depth + 1);
    return `z.array(${items})`;
  }

  if (type === "object") {
    const properties = schema.properties || {};
    const required = new Set(schema.required || []);

    if (Object.keys(properties).length === 0) {
      // Empty object - use record for flexibility
      return "z.object({})";
    }

    const indent = "  ".repeat(depth + 1);
    const closingIndent = "  ".repeat(depth);

    const props = Object.entries(properties).map(([key, prop]) => {
      let zodType = jsonSchemaToZod(prop, defs, depth + 1);
      if (!required.has(key)) {
        zodType += ".optional()";
      }
      return `${indent}${key}: ${zodType},`;
    });

    return `z.object({\n${props.join("\n")}\n${closingIndent}})`;
  }

  // Fallback
  return "z.unknown()";
}

function main() {
  const defs = schema.$defs || schema.definitions || {};
  const zodSchemas = [];
  const typeExports = [];

  // Generate Zod schemas for each type
  for (const name of typeOrder) {
    const defSchema = defs[name];
    if (!defSchema) {
      console.warn(`Warning: type ${name} not found in schema`);
      continue;
    }

    const zodSchema = jsonSchemaToZod(defSchema, defs, 0);
    zodSchemas.push(`export const ${name}Schema = ${zodSchema};`);
    typeExports.push(`export type ${name} = z.infer<typeof ${name}Schema>;`);
  }

  // Build the output file
  const output = `/* eslint-disable */
/**
 * This file was automatically generated from schema/protocol.json
 * DO NOT EDIT MANUALLY - run \`just client-types\` to regenerate
 */

import { z } from "zod/v4";

// =============================================================================
// Zod Schemas - validated against JSON Schema definitions
// =============================================================================

${zodSchemas.join("\n\n")}

// =============================================================================
// Inferred TypeScript types
// =============================================================================

${typeExports.join("\n")}

// =============================================================================
// Helper types for working with the protocol
// =============================================================================

/**
 * All valid message type strings
 */
export type MessageType =
  | "init"
  | "message"
  | "history"
  | "join_room"
  | "error";

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

// =============================================================================
// Runtime validation schemas for server envelopes
// =============================================================================

export const InitEnvelopeSchema = z.object({
  type: z.literal("init"),
  data: InitResponseSchema,
});

export const MessageEnvelopeSchema = z.object({
  type: z.literal("message"),
  data: MessageSchema,
});

export const HistoryEnvelopeSchema = z.object({
  type: z.literal("history"),
  data: HistoryResponseSchema,
});

export const JoinRoomEnvelopeSchema = z.object({
  type: z.literal("join_room"),
  data: JoinRoomResponseSchema,
});

export const ErrorEnvelopeSchema = z.object({
  type: z.literal("error"),
  data: ErrorResponseSchema,
});

/**
 * Discriminated union schema for all server → client messages
 */
export const ServerEnvelopeSchema = z.discriminatedUnion("type", [
  InitEnvelopeSchema,
  MessageEnvelopeSchema,
  HistoryEnvelopeSchema,
  JoinRoomEnvelopeSchema,
  ErrorEnvelopeSchema,
]);

/**
 * Type guard for checking message type
 */
export function isMessageType<T extends MessageType>(
  envelope: ServerEnvelope,
  type: T,
): envelope is Extract<ServerEnvelope, { type: T }> {
  return envelope.type === type;
}

/**
 * Parse and validate a server envelope from raw JSON
 * @throws z.ZodError if validation fails
 */
export function parseServerEnvelope(data: unknown): ServerEnvelope {
  return ServerEnvelopeSchema.parse(data);
}

/**
 * Safely parse a server envelope, returning null on failure
 */
export function safeParseServerEnvelope(data: unknown): ServerEnvelope | null {
  const result = ServerEnvelopeSchema.safeParse(data);
  return result.success ? result.data : null;
}
`;

  writeFileSync(outputPath, output);
  console.log(`Generated ${outputPath}`);
}

main();
