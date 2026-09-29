package service

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/dmykolen/meeting-transcriber-go/internal/home"
	"github.com/dmykolen/meeting-transcriber-go/internal/insights"
	"github.com/dmykolen/meeting-transcriber-go/internal/library"
	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

func TestToolsExposeStoredInformation(t *testing.T) {
	meetings, recordingID, projectID := testMeetings(t)
	s := NewMCP(meetings)
	// Every call below fails if its structured content breaks the tool's
	// declared output schema, as it does in clients that validate it.
	server.WithOutputSchemaValidation()(s)
	client, err := mcpclient.NewInProcessClient(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "test", Version: "1"}
	if _, err := client.Initialize(t.Context(), init); err != nil {
		t.Fatal(err)
	}

	listed, err := client.ListTools(t.Context(), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 9 {
		t.Fatalf("got %d tools, want 9", len(listed.Tools))
	}
	for _, tool := range listed.Tools {
		if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint ||
			tool.Annotations.DestructiveHint == nil || *tool.Annotations.DestructiveHint ||
			tool.Annotations.IdempotentHint == nil || !*tool.Annotations.IdempotentHint ||
			tool.Annotations.OpenWorldHint == nil || *tool.Annotations.OpenWorldHint {
			t.Errorf("%s has unsafe annotations: %+v", tool.Name, tool.Annotations)
		}
		if tool.OutputSchema.Type == "" {
			t.Errorf("%s has no output schema", tool.Name)
		}
	}

	var recordings recordingsOutput
	decodeResult(t, call(t, client, "list_recordings", map[string]any{}), &recordings)
	if len(recordings.Recordings) != 1 || recordings.Recordings[0].ID != recordingID {
		t.Fatalf("unexpected recordings: %+v", recordings.Recordings)
	}

	var recording recordingOutput
	decodeResult(t, call(t, client, "get_recording", map[string]any{"id": recordingID}), &recording)
	if len(recording.Recording.Transcript) != 2 || recording.Recording.Turns != 2 ||
		len(recording.Notes) != 1 || recording.Analytics.Words == 0 {
		t.Fatalf("incomplete recording: %+v", recording)
	}

	var knowledge knowledgeSearchOutput
	decodeResult(t, call(t, client, "search_knowledge", map[string]any{"query": "launch"}), &knowledge)
	if len(knowledge.Hits) == 0 {
		t.Fatal("knowledge search did not find stored summary")
	}

	var transcript transcriptSearchOutput
	decodeResult(t, call(t, client, "search_transcripts", map[string]any{"query": "launch"}), &transcript)
	if len(transcript.Hits) == 0 {
		t.Fatal("transcript search did not find stored turn")
	}

	var projects projectsOutput
	decodeResult(t, call(t, client, "list_projects", map[string]any{}), &projects)
	if len(projects.Projects) != 1 {
		t.Fatalf("unexpected projects: %+v", projects)
	}

	var project projectOutput
	decodeResult(t, call(t, client, "get_project", map[string]any{"id": projectID}), &project)
	if project.Project.ID != projectID || len(project.Recordings) != 1 || len(project.Notes) != 1 {
		t.Fatalf("incomplete project: %+v", project)
	}

	var people peopleOutput
	decodeResult(t, call(t, client, "list_people", map[string]any{}), &people)
	if len(people.People) != 1 || people.People[0].Name != "Alice" {
		t.Fatalf("unexpected people: %+v", people.People)
	}

	var actions actionsOutput
	decodeResult(t, call(t, client, "list_action_items", map[string]any{}), &actions)
	if len(actions.Actions) != 1 {
		t.Fatalf("unexpected actions: %+v", actions)
	}

	var briefing briefingOutput
	decodeResult(t, call(t, client, "get_briefing", map[string]any{"days": 1}), &briefing)
	if len(briefing.Briefing.Meetings) != 1 {
		t.Fatalf("unexpected briefing: %+v", briefing)
	}

	if result := call(t, client, "get_project", map[string]any{"id": int64(999)}); !result.IsError {
		t.Fatal("missing project should return a tool error")
	}
	if result := call(t, client, "list_recordings", map[string]any{
		"project_id": projectID, "include_deleted": true,
	}); !result.IsError {
		t.Fatal("conflicting list filters should return a tool error")
	}
}

func TestRunMCPRejectsNetworkAddress(t *testing.T) {
	meetings, _, _ := testMeetings(t)
	if err := RunMCP(t.Context(), meetings, "0.0.0.0:8765"); err == nil {
		t.Fatal("non-loopback MCP address should be rejected")
	}
	if state := meetings.MCPStatus(); state.Status != "failed" || state.Problem == "" {
		t.Fatalf("unexpected MCP state: %+v", state)
	}
}

func TestRunMCPServesStreamableHTTP(t *testing.T) {
	meetings, _, _ := testMeetings(t)
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	_ = probe.Close()

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- RunMCP(ctx, meetings, addr) }()

	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("MCP server did not listen on %s: %v", addr, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	for meetings.MCPStatus().Status != "running" {
		if time.Now().After(deadline) {
			t.Fatalf("unexpected running MCP state: %+v", meetings.MCPStatus())
		}
		time.Sleep(time.Millisecond)
	}
	if state := meetings.MCPStatus(); state.URL != "http://"+addr+"/mcp" || state.Command == "" {
		t.Fatalf("unexpected running MCP state: %+v", state)
	}

	client, err := mcpclient.NewStreamableHttpClient("http://" + addr + "/mcp")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "http-test", Version: "1"}
	if _, err := client.Initialize(t.Context(), init); err != nil {
		t.Fatal(err)
	}
	if tools, err := client.ListTools(t.Context(), mcp.ListToolsRequest{}); err != nil || len(tools.Tools) != 9 {
		t.Fatalf("HTTP tool discovery failed: tools=%v err=%v", tools, err)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("MCP server did not shut down")
	}
	if state := meetings.MCPStatus(); state.Status != "stopped" {
		t.Fatalf("unexpected stopped MCP state: %+v", state)
	}
}

func testMeetings(t *testing.T) (*Meetings, int64, int64) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(home.Database(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	project, err := db.NewGroup("Launch")
	if err != nil {
		t.Fatal(err)
	}
	recording, err := db.Add(store.Recording{
		Kind: store.Meeting, Title: "Launch review", Audio: "launch.wav",
		Started: time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Assign(recording.ID, project.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveTranscript(recording.ID, "en", 120, []store.Turn{
		{Start: 1, End: 4, Speaker: "Alice", Text: "The launch is ready."},
		{Start: 5, End: 8, Speaker: "Bob", Text: "Ship it tomorrow."},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSummary(recording.ID, &store.Summary{
		Title: "Launch review", Overview: "The launch is ready.",
		Decisions:   []string{"Launch tomorrow"},
		ActionItems: []store.Action{{Task: "Publish release", Owner: "Alice"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PutNote(store.Sticky{Recording: recording.ID, Text: "Meeting note"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.PutNote(store.Sticky{Project: project.ID, Text: "Project note"}); err != nil {
		t.Fatal(err)
	}
	if err := db.Remember("Alice", []float32{1, 0}, store.Source{
		Recording: recording.ID, Speaker: "Alice",
	}); err != nil {
		t.Fatal(err)
	}

	cfg := home.Defaults()
	lib := library.New(db, nil, insights.New(insights.Setup{Language: cfg.Language}), home.Recordings(dir))
	return New(db, lib, dir, cfg), recording.ID, project.ID
}

func call(t *testing.T, client *mcpclient.Client, name string, arguments map[string]any) *mcp.CallToolResult {
	t.Helper()
	request := mcp.CallToolRequest{}
	request.Params.Name = name
	request.Params.Arguments = arguments
	result, err := client.CallTool(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func decodeResult(t *testing.T, result *mcp.CallToolResult, target any) {
	t.Helper()
	if result.IsError {
		t.Fatalf("tool returned an error: %+v", result.Content)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
}
