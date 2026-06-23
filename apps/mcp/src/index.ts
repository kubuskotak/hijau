#!/usr/bin/env bun
// stdio entry point. Configure with HIJAU_API_URL (default http://localhost:8080)
// and HIJAU_TOKEN (a PAT). Logs go to stderr — stdout is the JSON-RPC channel.

import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { HijauClient } from './client';
import { createServer } from './server';

const base = (process.env.HIJAU_API_URL ?? 'http://localhost:8080').replace(/\/$/, '') + '/api/v1';
const token = process.env.HIJAU_TOKEN;

const server = createServer(new HijauClient(base, token));
await server.connect(new StdioServerTransport());
console.error(`hijau mcp ready · api=${base}${token ? '' : ' · WARNING: no HIJAU_TOKEN set'}`);
