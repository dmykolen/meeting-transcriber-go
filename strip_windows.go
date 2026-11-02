package main

import "github.com/wailsapp/wails/v3/pkg/application"

// untouchable keeps the strip above the meeting app. WebView2 hands the first
// click on an inactive window to the page, so Pause and Stop need nothing
// more; unlike the macOS panel, that click does make the strip active.
func untouchable(window *application.WebviewWindow) { window.SetAlwaysOnTop(true) }
