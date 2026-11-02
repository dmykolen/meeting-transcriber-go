package main

import (
	"os"

	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

// One copy runs at a time. macOS opens an app once however often it is
// clicked; Windows starts a process for every click, and a second one would
// open the same database and microphone and discard the recording the first is
// making as a stale partial file. So it brings the first one's window forward
// and leaves before it touches anything. The installer looks for the same
// mutex. The MCP server Claude Desktop starts is a second process by design.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "--mcp-stdio" {
		return
	}
	if _, err := windows.CreateMutex(nil, false, windows.StringToUTF16Ptr("MeetingTranscriber")); err != windows.ERROR_ALREADY_EXISTS {
		return
	}
	if w := w32.FindWindowW(windows.StringToUTF16Ptr("WailsWebviewWindow"), windows.StringToUTF16Ptr("Meeting Transcriber")); w != 0 {
		if !w32.IsZoomed(w) {
			w32.ShowWindow(w, w32.SW_RESTORE)
		}
		w32.SetForegroundWindow(w)
	}
	os.Exit(0)
}
