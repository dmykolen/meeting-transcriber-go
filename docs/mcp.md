# MCP access

The desktop app serves its stored meetings over Streamable HTTP while it is
running:

```text
http://127.0.0.1:8765/mcp
```

The endpoint is loopback-only and every tool is read-only. It exposes
transcripts, summaries, analytics, notes, projects, action items, briefings,
knowledge search, and recognised people. It never returns the OpenAI key or raw
voiceprint vectors.

Connect Claude Desktop by adding the following entry to
`~/Library/Application Support/Claude/claude_desktop_config.json` and
restarting Claude:

```json
{
  "mcpServers": {
    "meeting-transcriber": {
      "command": "/Applications/Meeting Transcriber.app/Contents/MacOS/MeetingTranscriber",
      "args": ["--mcp-stdio"]
    }
  }
}
```

Claude launches the same application executable in a headless stdio mode. No
second MCP executable is installed.

Connect Codex or the ChatGPT desktop app through **Settings → MCP servers →
Add server → Streamable HTTP**, or add this to `~/.codex/config.toml`:

```toml
[mcp_servers.meeting-transcriber]
url = "http://127.0.0.1:8765/mcp"
```

In VS Code, run **MCP: Add Server**, select **HTTP**, enter the URL above, and
save it to the user profile. The equivalent `mcp.json` is:

```json
{
  "servers": {
    "meeting-transcriber": {
      "type": "http",
      "url": "http://127.0.0.1:8765/mcp"
    }
  }
}
```

Set `MT_MCP_ADDR` before starting the app to use another loopback address, for
example `MT_MCP_ADDR=127.0.0.1:9876`. Non-loopback addresses are rejected so
the meeting archive cannot be exposed to the network accidentally.
