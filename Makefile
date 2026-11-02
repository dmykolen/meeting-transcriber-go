# Meeting Transcriber — the single-binary edition.
#
# Two native pieces are compiled in rather than shipped beside the binary:
# whisper.cpp for transcription and sherpa-onnx for speakers. whisper.cpp is
# built here from a pinned commit; sherpa-onnx ships prebuilt libraries in its
# Go module and needs nothing.
#
# The models are not compiled in and never will be — see internal/models.

WHISPER_REPO := https://github.com/ggml-org/whisper.cpp.git
WHISPER_REF  := eacbd8234c6654cdbf2c377f72b2106875479bdc

BUILD   := build
VENDOR  := $(BUILD)/whisper.cpp
WHISPER := $(abspath $(VENDOR))
APP     := $(BUILD)/Meeting Transcriber.app
INSTALL := /Applications/Meeting Transcriber.app
DMG     := $(BUILD)/MeetingTranscriber.dmg
MNT     := $(BUILD)/mnt

# audiotee is how the other participants are heard on macOS: miniaudio's
# loopback is WASAPI-only, so system audio has to come from Core Audio process
# taps, which are Objective-C. Running an MIT-licensed Swift CLI keeps that out
# of this process entirely — no cgo for it, and a crash in the tap cannot take
# the app down. It has no tagged releases and warns its API is unstable, so the
# commit is pinned rather than tracked.
AUDIOTEE_REPO := https://github.com/makeusabrew/audiotee.git
AUDIOTEE_REF  := 56ac954369a09318e46b88a6eec33c2d2b0d32a3
AUDIOTEE      := $(BUILD)/audiotee

# llama-server runs the local AI models when the settings choose them. It is a
# helper process for the same reasons as audiotee, and one more: linked in, its
# ggml would be a second copy beside whisper.cpp's. Pinned to release v0.5.0.
LLAMA_REPO := https://github.com/ggml-org/llama.cpp.git
LLAMA_REF  := 7fe450e19305b828c199d602c23a8337aaa1f03b
LLAMA      := $(BUILD)/llama-server

# The speaker models come with two dylibs that the linker resolves through an
# rpath into the Go module cache. That is fine on this machine and useless on
# anybody else's, so the bundle carries them and the rpath is rewritten.
SHERPA := $(shell go env GOMODCACHE)/github.com/k2-fsa/sherpa-onnx-go-macos@v1.13.7/lib/aarch64-apple-darwin

# A stable signing identity, if one has been made. Without it the bundle is
# ad-hoc signed, which works — but an ad-hoc signature contains a hash of the
# binary, macOS ties the microphone grant to that hash, and so every rebuild is
# a different application that has to ask again. `make cert` fixes that once.
# By fingerprint, not by name: a self-signed certificate is never "valid" for
# codesigning, and two with the same name make signing by name ambiguous.
IDENTITY := $(shell security find-identity -p codesigning 2>/dev/null | awk '/Meeting Transcriber Local/ {print $$2}' | sort | head -1)
SIGN     := $(if $(IDENTITY),$(IDENTITY),-)

export CPATH        := $(WHISPER)/include:$(WHISPER)/ggml/include
export LIBRARY_PATH := $(WHISPER)/build/src:$(WHISPER)/build/ggml/src:$(WHISPER)/build/ggml/src/ggml-metal:$(WHISPER)/build/ggml/src/ggml-blas

.PHONY: all whisper frontend bundle dmg run install cert test clean

all: $(BUILD)/mt

## whisper — build the transcription engine once; everything else depends on it
whisper: $(VENDOR)/build/src/libwhisper.a

$(VENDOR)/build/src/libwhisper.a:
	@mkdir -p $(BUILD)
	git clone --quiet $(WHISPER_REPO) $(VENDOR) 2>/dev/null || true
	cd $(VENDOR) && git fetch --quiet origin && git checkout --quiet $(WHISPER_REF)
	cd $(VENDOR) && cmake -B build -DCMAKE_BUILD_TYPE=Release -DGGML_METAL=ON \
		-DBUILD_SHARED_LIBS=OFF -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_EXAMPLES=OFF >/dev/null
	cd $(VENDOR) && cmake --build build -j $(shell sysctl -n hw.ncpu 2>/dev/null || nproc) >/dev/null
	@echo "whisper.cpp built at $(WHISPER_REF)"

$(BUILD)/mt: whisper frontend $(shell find . -name '*.go' 2>/dev/null)
	go build -o $@ .

## frontend — the interface, built into the binary by go:embed
frontend:
	cd frontend && npm install --silent && npm run build

# --- packaging --------------------------------------------------------------

$(AUDIOTEE):
	@mkdir -p $(BUILD)
	git clone --quiet $(AUDIOTEE_REPO) $(BUILD)/audiotee.src 2>/dev/null || true
	cd $(BUILD)/audiotee.src && git fetch --quiet origin $(AUDIOTEE_REF) && git checkout --quiet $(AUDIOTEE_REF)
	cd $(BUILD)/audiotee.src && swift build -c release
	cp $(BUILD)/audiotee.src/.build/release/audiotee $@

$(LLAMA):
	@mkdir -p $(BUILD)/llama.cpp
	cd $(BUILD)/llama.cpp && git init --quiet && git fetch --quiet --depth 1 $(LLAMA_REPO) $(LLAMA_REF) && git checkout --quiet FETCH_HEAD
	cd $(BUILD)/llama.cpp && cmake -B build -DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF \
		-DGGML_METAL=ON -DGGML_METAL_EMBED_LIBRARY=ON -DLLAMA_OPENSSL=OFF -DLLAMA_USE_PREBUILT_UI=OFF \
		-DLLAMA_BUILD_TESTS=OFF -DLLAMA_BUILD_EXAMPLES=OFF >/dev/null
	cd $(BUILD)/llama.cpp && cmake --build build --target llama-server -j $(shell sysctl -n hw.ncpu 2>/dev/null || nproc) >/dev/null
	cp $(BUILD)/llama.cpp/build/bin/llama-server $@

## bundle — the .app. macOS attaches the microphone and system-audio grants to
## this, so the app is always run from inside it, never as a bare binary.
bundle: $(BUILD)/mt $(AUDIOTEE) $(LLAMA)
	rm -rf "$(APP)"
	mkdir -p "$(APP)/Contents/MacOS" "$(APP)/Contents/Resources" "$(APP)/Contents/Frameworks"
	cp packaging/darwin/Info.plist "$(APP)/Contents/Info.plist"
	cp packaging/darwin/AppIcon.icns "$(APP)/Contents/Resources/AppIcon.icns"
	cp $(BUILD)/mt "$(APP)/Contents/MacOS/MeetingTranscriber"
	cp $(AUDIOTEE) "$(APP)/Contents/Resources/audiotee"
	cp $(LLAMA) "$(APP)/Contents/Resources/llama-server"
	cp $(SHERPA)/libsherpa-onnx-c-api.dylib $(SHERPA)/libonnxruntime.dylib "$(APP)/Contents/Frameworks/"
	chmod u+w "$(APP)/Contents/Frameworks/"*.dylib
	install_name_tool -add_rpath @executable_path/../Frameworks "$(APP)/Contents/MacOS/MeetingTranscriber"
	install_name_tool -delete_rpath $(SHERPA) "$(APP)/Contents/MacOS/MeetingTranscriber"
	codesign --force --sign $(SIGN) "$(APP)/Contents/Frameworks/libonnxruntime.dylib"
	codesign --force --sign $(SIGN) "$(APP)/Contents/Frameworks/libsherpa-onnx-c-api.dylib"
	codesign --force --sign $(SIGN) "$(APP)/Contents/Resources/audiotee"
	codesign --force --sign $(SIGN) "$(APP)/Contents/Resources/llama-server"
	codesign --force --sign $(SIGN) "$(APP)/Contents/MacOS/MeetingTranscriber"
	codesign --force --sign $(SIGN) "$(APP)"
	@codesign --verify --deep "$(APP)" && echo "bundle verified"
	@if [ -n "$(IDENTITY)" ]; then \
	  echo "signed by $(IDENTITY) — the microphone permission survives rebuilds"; \
	else \
	  echo "signed ad-hoc — macOS will ask for the microphone again after each rebuild."; \
	  echo "run 'make cert' once to stop that."; \
	fi

## dmg — what you hand to somebody who has never seen a terminal.
##
## Built read/write first so Finder can be told where the two icons go and what
## the window looks like, then compressed. A plain `hdiutil create` from a
## folder gives a window with two icons in a list and no hint about what to do
## with them, which is the one moment a first-time user is most likely to give
## up at.
dmg: bundle
	rm -rf $(BUILD)/dmg $(DMG) $(BUILD)/rw.dmg
	mkdir -p "$(BUILD)/dmg/.background"
	cp -R "$(APP)" $(BUILD)/dmg/
	cp packaging/darwin/background.tiff "$(BUILD)/dmg/.background/background.tiff"
	ln -s /Applications $(BUILD)/dmg/Applications
	hdiutil create -quiet -volname "Meeting Transcriber" -srcfolder $(BUILD)/dmg \
		-ov -format UDRW $(BUILD)/rw.dmg
	@# Mounted where Finder can see it: -nobrowse hides the volume from Finder
	@# entirely, and Finder is the only thing on macOS that can set a window's
	@# background and icon positions.
	@#
	@# The whole styling step is optional and must never fail the build. It
	@# needs permission to control Finder, which macOS grants once, to the
	@# application running make — and in a terminal with no one watching there
	@# is nobody to answer the dialog, so the AppleEvent simply times out.
	@# Without it the disk image is a plain window with two icons, which works.
	@hdiutil attach -quiet -mountpoint "$(MNT)" $(BUILD)/rw.dmg
	-@osascript packaging/darwin/dress-dmg.applescript "Meeting Transcriber" >/dev/null 2>&1 && \
		echo "disk image window styled" || \
		echo "note: the disk image opens as a plain window. To get the laid-out one," && \
		echo "      System Settings > Privacy & Security > Automation > your terminal >" && \
		echo "      turn on Finder, then run 'make dmg' again. The app itself is unaffected."
	@sync
	@hdiutil detach -quiet "$(MNT)" || hdiutil detach -quiet -force "$(MNT)"
	hdiutil convert -quiet $(BUILD)/rw.dmg -format UDZO -ov -o $(DMG)
	@rm -rf $(BUILD)/rw.dmg $(MNT)
	@codesign --force --sign $(SIGN) $(DMG)
	@echo "$(DMG)  $$(du -h $(DMG) | cut -f1) — drag the app onto Applications and open it"

## run — through `open`, which is what makes macOS treat the bundle as the
## responsible application. Started from a shell instead, the microphone grant
## is attributed to the terminal and the app records silence.
run: bundle
	open "$(APP)"

## install — the one copy the Dock and Spotlight should open. Every build is
## signed the same way, so the microphone and system-audio grants carry over. A
## running copy keeps working on the files it already has; quit and reopen it to
## run this build.
install: bundle
	rm -rf "$(INSTALL)"
	ditto "$(APP)" "$(INSTALL)"
	@codesign --verify --deep "$(INSTALL)" && echo "installed $(INSTALL) — quit and reopen the app to run it"

## cert — a local signing identity, so the permission is asked for once
cert:
	@bash packaging/darwin/make-signing-identity.sh

test: whisper
	go vet ./...
	go test ./...

clean:
	rm -rf $(BUILD)

# --- Windows ----------------------------------------------------------------
#
# The same app as a Windows installer, cross-compiled on this Mac after
# `brew install mingw-w64 makensis`. Nothing above reads anything from here:
# whisper.cpp and llama.cpp get a build directory of their own beside the
# macOS one, and the Windows files live in build/windows.
#
# x86-64 only: sherpa-onnx ships no ARM64 Windows libraries. AVX2 is the floor,
# since GGML_NATIVE would tune for this Mac and plain SSE2 is too slow for
# Whisper; the installer refuses a processor without it. OpenMP is off, so ggml
# uses its own threads and no libgomp ships.

WIN      := $(BUILD)/windows
SETUP    := $(BUILD)/MeetingTranscriber-Setup.exe
MINGW    := x86_64-w64-mingw32
WINCMAKE := -DCMAKE_SYSTEM_NAME=Windows -DCMAKE_SYSTEM_PROCESSOR=x86_64 \
	-DCMAKE_C_COMPILER=$(MINGW)-gcc -DCMAKE_CXX_COMPILER=$(MINGW)-g++ -DCMAKE_RC_COMPILER=$(MINGW)-windres \
	-DCMAKE_BUILD_TYPE=Release -DBUILD_SHARED_LIBS=OFF -DGGML_OPENMP=OFF -DGGML_NATIVE=OFF \
	-DGGML_SSE42=ON -DGGML_AVX=ON -DGGML_AVX2=ON -DGGML_FMA=ON -DGGML_F16C=ON -DGGML_BMI2=ON

# Recursive, so a macOS build never spends time working these out. The Wails
# CLI runs at the version go.mod pins and need not be installed.
VERSION    = $(shell sed -n 's/^const Version = "\(.*\)"/\1/p' internal/service/mcp.go)
WAILS3     = go run github.com/wailsapp/wails/v3/cmd/wails3@$(shell go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)
SHERPA_WIN = $(shell go env GOMODCACHE)/github.com/k2-fsa/sherpa-onnx-go-windows@v1.13.7/lib/x86_64-pc-windows-gnu

.PHONY: exe

$(VENDOR)/build-windows/src/libwhisper.a:
	@mkdir -p $(BUILD)
	git clone --quiet $(WHISPER_REPO) $(VENDOR) 2>/dev/null || true
	cd $(VENDOR) && git fetch --quiet origin && git checkout --quiet $(WHISPER_REF)
	cd $(VENDOR) && cmake -B build-windows $(WINCMAKE) -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_EXAMPLES=OFF >/dev/null
	cd $(VENDOR) && cmake --build build-windows -j $(shell sysctl -n hw.ncpu 2>/dev/null || nproc) >/dev/null

# ggml leaves the lib prefix off its Windows libraries, which the -l flags of
# the Go bindings expect. The static libstdc++ and winpthread sit beside them,
# so the linker takes them before the toolchain's DLL import libraries and the
# executable needs no mingw DLL. A cross gcc ignores LIBRARY_PATH, hence -L.
$(WIN)/lib/libwhisper.a: $(VENDOR)/build-windows/src/libwhisper.a
	@mkdir -p $(WIN)/lib
	for l in ggml ggml-base ggml-cpu; do cp $(VENDOR)/build-windows/ggml/src/$$l.a $(WIN)/lib/lib$$l.a; done
	cp "$$($(MINGW)-gcc -print-file-name=libstdc++.a)" "$$($(MINGW)-gcc -print-file-name=libpthread.a)" $(WIN)/lib/
	cp $< $@

# The .syso puts the icon, the manifest and the version into the executable; go
# build takes it from the package folder for windows/amd64 only. The old binary
# goes first: go build does not notice that a C library changed. The frontend
# target always runs, so this does too, and needs no list of Go files.
$(WIN)/MeetingTranscriber.exe: frontend $(WIN)/lib/libwhisper.a
	printf '{"fixed":{"file_version":"%s.0","product_version":"%s.0"},"info":{"0409":{"ProductName":"Meeting Transcriber","FileDescription":"Meeting Transcriber","ProductVersion":"%s","FileVersion":"%s","LegalCopyright":"Copyright (c) 2026 Dmytro Mykolenko"}}}' \
		$(VERSION) $(VERSION) $(VERSION) $(VERSION) > $(WIN)/info.json
	$(WAILS3) generate syso -arch amd64 -icon packaging/windows/icon.ico \
		-manifest packaging/windows/MeetingTranscriber.exe.manifest -info $(WIN)/info.json -out rsrc_windows_amd64.syso
	rm -f $@
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(MINGW)-gcc CXX=$(MINGW)-g++ CGO_LDFLAGS=-L$(abspath $(WIN)/lib) \
		go build -ldflags "-H windowsgui -extldflags -static-libgcc" -o $@ . ; s=$$?; rm -f rsrc_windows_amd64.syso; exit $$s

$(WIN)/llama-server.exe:
	@mkdir -p $(BUILD)/llama.cpp $(WIN)
	cd $(BUILD)/llama.cpp && git init --quiet && git fetch --quiet --depth 1 $(LLAMA_REPO) $(LLAMA_REF) && git checkout --quiet FETCH_HEAD
	cd $(BUILD)/llama.cpp && cmake -B build-windows $(WINCMAKE) -DCMAKE_EXE_LINKER_FLAGS=-static \
		-DLLAMA_OPENSSL=OFF -DLLAMA_USE_PREBUILT_UI=OFF -DLLAMA_BUILD_TESTS=OFF -DLLAMA_BUILD_EXAMPLES=OFF >/dev/null
	cd $(BUILD)/llama.cpp && cmake --build build-windows --target llama-server -j $(shell sysctl -n hw.ncpu 2>/dev/null || nproc) >/dev/null
	cp $(BUILD)/llama.cpp/build-windows/bin/llama-server.exe $@

## exe — the Windows installer: the app, sherpa-onnx's two DLLs, llama-server,
## and Microsoft's WebView2 bootstrapper for a Windows 10 that lacks the runtime.
exe: $(SETUP)

$(SETUP): $(WIN)/MeetingTranscriber.exe $(WIN)/llama-server.exe packaging/windows/installer.nsi
	cp -f $(SHERPA_WIN)/sherpa-onnx-c-api.dll $(SHERPA_WIN)/onnxruntime.dll $(WIN)/
	chmod u+w $(WIN)/*.dll
	$(WAILS3) generate webview2bootstrapper -dir $(WIN)
	@# makensis 3.13 aborts with bad_alloc when the shell names no UTF-8 locale.
	LC_CTYPE=en_US.UTF-8 makensis -V2 -DVERSION=$(VERSION) -DSOURCE=$(abspath $(WIN)) -DOUT=$(abspath $@) packaging/windows/installer.nsi
	@echo "$@  $$(du -h $@ | cut -f1) — run it on Windows; it installs for the person running it"
