#!/usr/bin/env bun
// MCP entry point with two transports:
//   - stdio (default): for local assistants (Claude Desktop, Cursor, etc.).
//     Configure HIJAU_API_URL + HIJAU_TOKEN (a PAT). stdout is the JSON-RPC
//     channel, so all logs go to stderr.
//   - Streamable HTTP (`--http`, or set HIJAU_MCP_HTTP=<port>): for remote
//     assistants. POST JSON-RPC to /mcp; each request authenticates with its own
//     `Authorization: Bearer <PAT>` (falling back to HIJAU_TOKEN), so one server
//     can serve many users. Stateless — a fresh MCP server per request.

import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import { WebStandardStreamableHTTPServerTransport } from '@modelcontextprotocol/sdk/server/webStandardStreamableHttp.js';
import { HijauClient } from './client';
import { createServer } from './server';

const base = (process.env.HIJAU_API_URL ?? 'http://localhost:8080').replace(/\/$/, '') + '/api/v1';
const envToken = process.env.HIJAU_TOKEN;

const useHTTP = process.argv.includes('--http') || !!process.env.HIJAU_MCP_HTTP;

if (useHTTP) {
	const port = Number(process.env.HIJAU_MCP_HTTP) || Number(process.env.PORT) || 8765;
	// Bind loopback by default (DNS-rebinding protection is left off because the
	// listener isn't LAN-reachable). Set HIJAU_MCP_HOST to widen it deliberately.
	const host = process.env.HIJAU_MCP_HOST ?? '127.0.0.1';
	Bun.serve({
		port,
		hostname: host,
		async fetch(req) {
			if (new URL(req.url).pathname !== '/mcp') {
				return new Response('not found', { status: 404 });
			}
			// Stateless single-shot: only POST carries a JSON-RPC request. Reject GET
			// (a stateless GET-SSE stream would never close, pinning a server) + DELETE.
			if (req.method !== 'POST') {
				return Response.json(
					{ jsonrpc: '2.0', error: { code: -32000, message: 'Method Not Allowed; POST JSON-RPC to /mcp' }, id: null },
					{ status: 405, headers: { Allow: 'POST' } }
				);
			}
			// Per-request auth: the caller's own PAT (multi-user), else the env token.
			const token = (req.headers.get('authorization') ?? '').replace(/^Bearer\s+/i, '').trim() || envToken;
			if (!token) {
				return Response.json({ error: 'missing Authorization: Bearer <token>' }, { status: 401 });
			}
			const server = createServer(new HijauClient(base, token));
			const transport = new WebStandardStreamableHTTPServerTransport({
				sessionIdGenerator: undefined, // stateless: one server+transport per request
				enableJsonResponse: true
			});
			await server.connect(transport);
			return transport.handleRequest(req);
		},
		error(e) {
			console.error('mcp http error:', e);
			return Response.json(
				{ jsonrpc: '2.0', error: { code: -32603, message: 'Internal error' }, id: null },
				{ status: 500 }
			);
		}
	});
	console.error(
		`hijau mcp (streamable http) on ${host}:${port}/mcp · api=${base}` +
			(envToken ? ' · WARNING: HIJAU_TOKEN set — requests without a Bearer header run as the server PAT' : '')
	);
} else {
	const server = createServer(new HijauClient(base, envToken));
	await server.connect(new StdioServerTransport());
	console.error(`hijau mcp (stdio) ready · api=${base}${envToken ? '' : ' · WARNING: no HIJAU_TOKEN set'}`);
}
