package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// fit maximises the window when its screen is smaller than the window. macOS
// keeps a window on its screen; Windows centres it and leaves the title bar
// above the top edge, on a 1366×768 laptop or a 1080p one scaled to 150%.
func fit(window *application.WebviewWindow) {
	window.OnWindowEvent(events.Windows.WindowShow, func(*application.WindowEvent) {
		if s, err := window.GetScreen(); err == nil {
			if w, h := window.Size(); w > s.WorkArea.Width || h > s.WorkArea.Height {
				window.Maximise()
			}
		}
	})
}
