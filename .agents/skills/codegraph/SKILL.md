---
name: codegraph
description: Fast codebase knowledge graph for symbol lookup, call paths, callers/callees, and impact analysis in this project.
---

# CodeGraph Integration

CodeGraph is active for this project via MCP and local CLI.

## MCP Server
- Configured in `~/.gemini/config/mcp_config.json`.
- Exposes tools for AST symbol exploration, caller/callee trees, and impact analysis without manual scanning.

## CLI Usage in `d:\accounting`
- `codegraph query <symbol>`: Search symbols, classes, methods, and properties.
- `codegraph callers <symbol>`: Find all callers of a method/symbol.
- `codegraph callees <symbol>`: Find all dependencies called by a symbol.
- `codegraph impact <symbol>`: Trace the blast radius of changes to a symbol.
- `codegraph sync`: Sync recent changes to `.codegraph/codegraph.db`.
- `codegraph status`: Inspect index node/edge stats and health.
