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
