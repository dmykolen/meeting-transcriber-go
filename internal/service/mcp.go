package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/dmykolen/meeting-transcriber-go/internal/store"
)

const DefaultAddr = "127.0.0.1:8765"

// MCPState is the local server status shown in Settings.
type MCPState struct {
	Status  string `json:"status"`
	URL     string `json:"url"`
	Command string `json:"command"`
	Problem string `json:"problem,omitempty"`
}

type empty struct{}

type listRecordingsInput struct {
	Limit          int   `json:"limit,omitempty" jsonschema:"Maximum recordings to return, from 1 to 200 (default 100)"`
	ProjectID      int64 `json:"project_id,omitempty" jsonschema:"Only recordings filed in this project"`
	IncludeDeleted bool  `json:"include_deleted,omitempty" jsonschema:"List recordings in the bin instead of active recordings"`
}

type recordingsOutput struct {
	Recordings []store.Recording `json:"recordings"`
}

type recordingInput struct {
	ID int64 `json:"id" jsonschema:"Recording ID"`
}

type recordingOutput struct {
	Recording *Meeting         `json:"recording"`
	Analytics *store.Analytics `json:"analytics"`
	Notes     []store.Sticky   `json:"notes"`
}

type searchInput struct {
	Query string `json:"query" jsonschema:"Words or question to find"`
}

type transcriptSearchOutput struct {
	Hits []store.Hit `json:"hits"`
}

type knowledgeSearchInput struct {
	Query string `json:"query" jsonschema:"Words to find across transcripts, summaries, actions, notes, and projects"`
}

type knowledgeSearchOutput struct {
	Hits []store.KnowledgeHit `json:"hits"`
}

type projectsOutput struct {
	Projects []store.Group `json:"projects"`
	Unfiled  int           `json:"unfiled"`
}

type projectInput struct {
	ID int64 `json:"id" jsonschema:"Project ID"`
}

type projectOutput struct {
	Project    store.Group       `json:"project"`
	Standing   *store.Standing   `json:"standing"`
	Recordings []store.Recording `json:"recordings"`
	Notes      []store.Sticky    `json:"notes"`
}

type actionsInput struct {
	IncludeDone bool `json:"include_done,omitempty" jsonschema:"Include completed action items"`
}

type actionsOutput struct {
	Actions []store.Outstanding `json:"actions"`
}

type briefingInput struct {
	Days int `json:"days,omitempty" jsonschema:"Number of recent days to cover, from 1 to 365 (default 1)"`
}

type briefingOutput struct {
	Briefing *store.Briefing `json:"briefing"`
}

type peopleOutput struct {
	People []store.Person `json:"people"`
}

// Version is the app's version, as the MCP server and the log report it.
const Version = "1.4.0"

func NewMCP(meetings *Meetings) *server.MCPServer {
	s := server.NewMCPServer(
		"Meeting Transcriber",
		Version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	addTool(s, "list_recordings", "List saved meetings and voice notes, optionally within one project or from the bin.",
		func(ctx context.Context, _ mcp.CallToolRequest, in listRecordingsInput) (recordingsOutput, error) {
			if err := ctx.Err(); err != nil {
				return recordingsOutput{}, err
			}
			if in.ProjectID < 0 {
				return recordingsOutput{}, errors.New("project_id must be positive")
			}
			if in.ProjectID > 0 && in.IncludeDeleted {
				return recordingsOutput{}, errors.New("project_id and include_deleted cannot be combined")
			}
			if in.Limit <= 0 {
				in.Limit = 100
			}
			if in.Limit > 200 {
				return recordingsOutput{}, errors.New("limit cannot exceed 200")
			}
			var (
				found []store.Recording
				err   error
			)
			switch {
			case in.IncludeDeleted:
				found, err = meetings.Bin()
			case in.ProjectID > 0:
				found, err = meetings.InGroup(in.ProjectID)
			default:
				found, err = meetings.Recent(in.Limit)
			}
			if err != nil {
				return recordingsOutput{}, err
			}
			if len(found) > in.Limit {
				found = found[:in.Limit]
			}
			if found == nil {
				found = []store.Recording{}
			}
			return recordingsOutput{Recordings: found}, nil
		})

	addTool(s, "get_recording", "Get one recording with its full transcript, summary, notes, speakers, and conversation analytics.",
		func(ctx context.Context, _ mcp.CallToolRequest, in recordingInput) (recordingOutput, error) {
			if err := positiveID(ctx, in.ID, "recording"); err != nil {
				return recordingOutput{}, err
			}
			recording, err := meetings.Open(in.ID)
			if err != nil {
				return recordingOutput{}, err
			}
			analytics, err := meetings.Analytics(in.ID)
			if err != nil {
				return recordingOutput{}, err
			}
			notes, err := meetings.Notes(in.ID, 0)
			if err != nil {
				return recordingOutput{}, err
			}
			return recordingOutput{Recording: recording, Analytics: analytics, Notes: notes}, nil
		})

	addTool(s, "search_transcripts", "Find timestamped transcript passages containing every supplied keyword.",
		func(ctx context.Context, _ mcp.CallToolRequest, in searchInput) (transcriptSearchOutput, error) {
			if err := nonemptyQuery(ctx, in.Query); err != nil {
				return transcriptSearchOutput{}, err
			}
			hits, err := meetings.db.Search(strings.TrimSpace(in.Query), 40)
			if hits == nil {
				hits = []store.Hit{}
			}
			return transcriptSearchOutput{Hits: hits}, err
		})

	addTool(s, "search_knowledge", "Search all stored knowledge: transcripts, summaries, decisions, actions, questions, notes, and project documents.",
		func(ctx context.Context, _ mcp.CallToolRequest, in knowledgeSearchInput) (knowledgeSearchOutput, error) {
			if err := nonemptyQuery(ctx, in.Query); err != nil {
				return knowledgeSearchOutput{}, err
			}
			hits, err := meetings.SearchKnowledge(strings.TrimSpace(in.Query), false)
			if hits == nil {
				hits = []store.KnowledgeHit{}
			}
			return knowledgeSearchOutput{Hits: hits}, err
		})

	addTool(s, "list_projects", "List projects with their recording counts and the number of unfiled recordings.",
		func(ctx context.Context, _ mcp.CallToolRequest, _ empty) (projectsOutput, error) {
			if err := ctx.Err(); err != nil {
				return projectsOutput{}, err
			}
			projects, err := meetings.Groups()
			if err != nil {
				return projectsOutput{}, err
			}
			unfiled, err := meetings.Loose()
			return projectsOutput{Projects: projects, Unfiled: unfiled}, err
		})

	addTool(s, "get_project", "Get a project's current status, work, decisions, questions, people, recordings, and notes.",
		func(ctx context.Context, _ mcp.CallToolRequest, in projectInput) (projectOutput, error) {
			if err := positiveID(ctx, in.ID, "project"); err != nil {
				return projectOutput{}, err
			}
			groups, err := meetings.Groups()
			if err != nil {
				return projectOutput{}, err
			}
			var project *store.Group
			for i := range groups {
				if groups[i].ID == in.ID {
					project = &groups[i]
					break
				}
			}
			if project == nil {
				return projectOutput{}, fmt.Errorf("project %d was not found", in.ID)
			}
			standing, err := meetings.Standing(in.ID)
			if err != nil {
				return projectOutput{}, err
			}
			recordings, err := meetings.InGroup(in.ID)
			if err != nil {
				return projectOutput{}, err
			}
			notes, err := meetings.Notes(0, in.ID)
			if err != nil {
				return projectOutput{}, err
			}
			return projectOutput{Project: *project, Standing: standing, Recordings: recordings, Notes: notes}, nil
		})

	addTool(s, "list_action_items", "List action items across saved meetings.",
		func(ctx context.Context, _ mcp.CallToolRequest, in actionsInput) (actionsOutput, error) {
			if err := ctx.Err(); err != nil {
				return actionsOutput{}, err
			}
			actions, err := meetings.Actions(in.IncludeDone)
			if actions == nil {
				actions = []store.Outstanding{}
			}
			return actionsOutput{Actions: actions}, err
		})

	addTool(s, "get_briefing", "Get a recent cross-meeting briefing with decisions, open and overdue work, recurring questions, and participants.",
		func(ctx context.Context, _ mcp.CallToolRequest, in briefingInput) (briefingOutput, error) {
			if err := ctx.Err(); err != nil {
				return briefingOutput{}, err
			}
			if in.Days <= 0 {
				in.Days = 1
			}
			if in.Days > 365 {
				return briefingOutput{}, errors.New("days cannot exceed 365")
			}
			briefing, err := meetings.Brief(in.Days)
			return briefingOutput{Briefing: briefing}, err
		})

	addTool(s, "list_people", "List recognised people and the recordings their saved voice samples came from. Voiceprint vectors are never exposed.",
		func(ctx context.Context, _ mcp.CallToolRequest, _ empty) (peopleOutput, error) {
			if err := ctx.Err(); err != nil {
				return peopleOutput{}, err
			}
			people, err := meetings.People()
			if people == nil {
				people = []store.Person{}
			}
			return peopleOutput{People: people}, err
		})
	return s
}

func addTool[I, O any](s *server.MCPServer, name, description string, handler mcp.StructuredToolHandlerFunc[I, O]) {
	s.AddTool(mcp.NewTool(
		name,
		mcp.WithDescription(description),
		mcp.WithInputSchema[I](),
		mcp.WithOutputSchema[O](),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	), mcp.NewStructuredToolHandler(handler))
}

func positiveID(ctx context.Context, id int64, kind string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id <= 0 {
		return fmt.Errorf("%s id must be positive", kind)
	}
	return nil
}

func nonemptyQuery(ctx context.Context, query string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(query) == "" {
		return errors.New("query is required")
	}
	return nil
}

// MCPStatus reports whether external AI clients can reach the app.
func (m *Meetings) MCPStatus() MCPState {
	m.mcpMu.RLock()
	state := m.mcp
	m.mcpMu.RUnlock()
	if state.Status == "" {
		state.Status = "starting"
	}
	if state.Command == "" {
		state.Command, _ = os.Executable()
	}
	return state
}

func (m *Meetings) setMCP(status, url string, err error) {
	state := MCPState{Status: status, URL: url}
	if err != nil {
		state.Problem = err.Error()
	}
	m.mcpMu.Lock()
	m.mcp = state
	m.mcpMu.Unlock()
}

func RunMCP(ctx context.Context, meetings *Meetings, addr string) error {
	if addr == "" {
		addr = DefaultAddr
	}
	url := "http://" + addr + "/mcp"
	meetings.setMCP("starting", url, nil)
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		err = fmt.Errorf("invalid MT_MCP_ADDR %q: %w", addr, err)
		meetings.setMCP("failed", url, err)
		return err
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		err = fmt.Errorf("MT_MCP_ADDR must use a loopback host, got %q", host)
		meetings.setMCP("failed", url, err)
		return err
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		meetings.setMCP("failed", url, err)
		return err
	}
	transport := server.NewStreamableHTTPServer(NewMCP(meetings), server.WithStateLess(true))
	httpServer := &http.Server{Handler: transport, ReadHeaderTimeout: 5 * time.Second}
	stopped := make(chan error, 1)
	go func() { stopped <- httpServer.Serve(listener) }()
	meetings.setMCP("running", url, nil)
	slog.Info("MCP server ready", "url", url)

	select {
	case err := <-stopped:
		if errors.Is(err, http.ErrServerClosed) {
			meetings.setMCP("stopped", url, nil)
			return nil
		}
		meetings.setMCP("failed", url, err)
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := httpServer.Shutdown(shutdown)
		if err != nil {
			meetings.setMCP("failed", url, err)
			return err
		}
		meetings.setMCP("stopped", url, nil)
		return nil
	}
}

// ServeMCPStdio lets clients such as Claude Desktop launch this same binary
// without starting the desktop window.
func ServeMCPStdio(meetings *Meetings) error {
	return server.ServeStdio(NewMCP(meetings))
}
