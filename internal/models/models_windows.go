package models

// The helpers are executables, so each platform downloads its own build.
var (
	// ffmpeg covers containers the platform decoders do not handle. Windows
	// has no afconvert, so every format but WAV goes through it.
	FFmpeg = Model{
		Name:  "Format support",
		Key:   "ffmpeg.exe",
		URL:   "https://github.com/eugeneware/ffmpeg-static/releases/download/b6.1.1/ffmpeg-win32-x64.gz",
		Bytes: 29_581_307,
		Check: "8883a3dffbd0a16cf4ef95206ea05283f78908dbfb118f73c83f4951dcc06d77",
		Run:   true,
	}

	// Copilot is the GitHub Copilot CLI that the Copilot SDK drives, pinned to
	// the version SDK v1.0.28 was released against.
	Copilot = Model{
		Name:  "GitHub Copilot",
		Key:   "copilot.exe",
		URL:   "https://github.com/github/copilot-cli/releases/download/v1.0.105/copilot-win32-x64.zip",
		Bytes: 112_637_935,
		Check: "34e16e9cf47de0e1534920171d6f39fa246638bddbb6ece820870b382bcc061a",
		Run:   true,
	}
)
