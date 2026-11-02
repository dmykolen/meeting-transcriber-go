//go:build !windows

package models

// The helpers are executables, so each platform downloads its own build.
var (
	// ffmpeg covers containers the platform decoders do not handle.
	FFmpeg = Model{
		Name:  "Format support",
		Key:   "ffmpeg",
		URL:   "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-darwin-arm64.gz",
		Bytes: 19_246_198,
		Check: "8923876afa8db5585022d7860ec7e589af192f441c56793971276d450ed3bbfa",
		Run:   true,
	}

	// Copilot is the GitHub Copilot CLI that the Copilot SDK drives, pinned to
	// the version SDK v1.0.28 was released against.
	Copilot = Model{
		Name:  "GitHub Copilot",
		Key:   "copilot",
		URL:   "https://github.com/github/copilot-cli/releases/download/v1.0.105/copilot-darwin-arm64.tar.gz",
		Bytes: 119_263_282,
		Check: "34aacd03fb478d47e4b3cfc5ca045c37278c5d8d86239466521a16ea54ac84aa",
		Run:   true,
	}
)
