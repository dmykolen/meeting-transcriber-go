package main

import "github.com/wailsapp/wails/v3/pkg/application"

// fit has nothing to do on macOS, which keeps a window on its screen itself.
func fit(*application.WebviewWindow) {}
