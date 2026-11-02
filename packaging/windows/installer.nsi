; The Windows installer, made by `make exe`, which passes VERSION, SOURCE (the
; folder holding what is installed) and OUT. It installs for the person running
; it, with no administrator, much as the app is dragged into Applications on a
; Mac, and it leaves ~\MeetingTranscriber alone both ways.
Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"
!include "WinVer.nsh"
!include "FileFunc.nsh"

!define NAME "Meeting Transcriber"
!define EXE "MeetingTranscriber.exe"
!define UNINSTALL "Software\Microsoft\Windows\CurrentVersion\Uninstall\MeetingTranscriber"
; Where Microsoft's own installers record the WebView2 runtime.
!define WEBVIEW2 "Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"

Name "${NAME}"
OutFile "${OUT}"
RequestExecutionLevel user
InstallDir "$LOCALAPPDATA\Programs\${NAME}"
BrandingText " "

!define MUI_ICON "icon.ico"
!define MUI_UNICON "icon.ico"
!define MUI_FINISHPAGE_RUN "$INSTDIR\${EXE}"
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "Ukrainian"

VIProductVersion "${VERSION}.0"
VIFileVersion "${VERSION}.0"
VIAddVersionKey /LANG=${LANG_ENGLISH} "ProductName" "${NAME}"
VIAddVersionKey /LANG=${LANG_ENGLISH} "FileDescription" "${NAME} Setup"
VIAddVersionKey /LANG=${LANG_ENGLISH} "ProductVersion" "${VERSION}"
VIAddVersionKey /LANG=${LANG_ENGLISH} "FileVersion" "${VERSION}"
VIAddVersionKey /LANG=${LANG_ENGLISH} "LegalCopyright" "Copyright (c) 2026 Dmytro Mykolenko"

LangString Old ${LANG_ENGLISH} "${NAME} needs 64-bit Windows 10, version 2004 or later."
LangString Old ${LANG_UKRAINIAN} "${NAME} потребує 64-розрядної Windows 10 версії 2004 або новішої."
LangString NoAVX2 ${LANG_ENGLISH} "This computer's processor has no AVX2, and transcription cannot run without it."
LangString NoAVX2 ${LANG_UKRAINIAN} "Процесор цього комп'ютера не підтримує AVX2, а без нього розшифровка не працює."
LangString Running ${LANG_ENGLISH} "${NAME} is running. Quit it and try again."
LangString Running ${LANG_UKRAINIAN} "${NAME} працює. Закрийте його й спробуйте ще раз."
LangString WebView ${LANG_ENGLISH} "Installing Microsoft Edge WebView2, which draws the app's window"
LangString WebView ${LANG_UKRAINIAN} "Встановлюю Microsoft Edge WebView2, яким застосунок малює своє вікно"
LangString Kept ${LANG_ENGLISH} "Recordings, transcripts and settings stay in $PROFILE\MeetingTranscriber."
LangString Kept ${LANG_UKRAINIAN} "Записи, розшифровки й налаштування лишаються в $PROFILE\MeetingTranscriber."

; The app holds this mutex while it runs (instance_windows.go); its files cannot
; be replaced or removed until it quits.
!macro WaitForQuit
	again:
	System::Call 'kernel32::OpenMutexW(i 0x00100000, i 0, w "MeetingTranscriber") i .r0'
	${If} $0 <> 0
		System::Call 'kernel32::CloseHandle(i r0)'
		MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "$(Running)" /SD IDCANCEL IDRETRY again
		Abort
	${EndIf}
!macroend

Function .onInit
	; PF_AVX2_INSTRUCTIONS_AVAILABLE is answered from build 19041 on. Whisper is
	; built for AVX2 and would stop on its first instruction without it.
	${IfNot} ${RunningX64}
	${OrIfNot} ${AtLeastBuild} 19041
		MessageBox MB_ICONSTOP "$(Old)" /SD IDOK
		Abort
	${EndIf}
	System::Call 'kernel32::IsProcessorFeaturePresent(i 40) i .r0'
	${If} $0 = 0
		MessageBox MB_ICONSTOP "$(NoAVX2)" /SD IDOK
		Abort
	${EndIf}
FunctionEnd

Section
	!insertmacro WaitForQuit

	; Windows 11 ships WebView2. A Windows 10 without it gets Microsoft's small
	; bootstrapper, which downloads the runtime.
	SetRegView 64
	ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\${WEBVIEW2}" "pv"
	${If} $0 == ""
	${OrIf} $0 == "0.0.0.0"
		ReadRegStr $0 HKCU "Software\${WEBVIEW2}" "pv"
	${EndIf}
	${If} $0 == ""
	${OrIf} $0 == "0.0.0.0"
		DetailPrint "$(WebView)"
		InitPluginsDir
		File "/oname=$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe" "${SOURCE}\MicrosoftEdgeWebview2Setup.exe"
		ExecWait '"$PLUGINSDIR\MicrosoftEdgeWebview2Setup.exe" /silent /install'
	${EndIf}

	SetOutPath "$INSTDIR"
	File "${SOURCE}\${EXE}"
	File "${SOURCE}\sherpa-onnx-c-api.dll"
	File "${SOURCE}\onnxruntime.dll"
	File "${SOURCE}\llama-server.exe"
	WriteUninstaller "$INSTDIR\uninstall.exe"
	CreateShortcut "$SMPROGRAMS\${NAME}.lnk" "$INSTDIR\${EXE}"
	CreateShortcut "$DESKTOP\${NAME}.lnk" "$INSTDIR\${EXE}"

	WriteRegStr HKCU "${UNINSTALL}" "DisplayName" "${NAME}"
	WriteRegStr HKCU "${UNINSTALL}" "DisplayVersion" "${VERSION}"
	WriteRegStr HKCU "${UNINSTALL}" "Publisher" "Dmytro Mykolenko"
	WriteRegStr HKCU "${UNINSTALL}" "DisplayIcon" "$INSTDIR\${EXE}"
	WriteRegStr HKCU "${UNINSTALL}" "UninstallString" '"$INSTDIR\uninstall.exe"'
	WriteRegStr HKCU "${UNINSTALL}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
	WriteRegDWORD HKCU "${UNINSTALL}" "NoModify" 1
	WriteRegDWORD HKCU "${UNINSTALL}" "NoRepair" 1
	${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
	WriteRegDWORD HKCU "${UNINSTALL}" "EstimatedSize" $0
SectionEnd

Section "Uninstall"
	!insertmacro WaitForQuit
	Delete "$INSTDIR\${EXE}"
	Delete "$INSTDIR\sherpa-onnx-c-api.dll"
	Delete "$INSTDIR\onnxruntime.dll"
	Delete "$INSTDIR\llama-server.exe"
	Delete "$INSTDIR\uninstall.exe"
	RMDir "$INSTDIR"
	Delete "$SMPROGRAMS\${NAME}.lnk"
	Delete "$DESKTOP\${NAME}.lnk"
	; WebView2's own data for the app: its cache and the window's small preferences.
	RMDir /r "$APPDATA\${EXE}"
	DeleteRegKey HKCU "${UNINSTALL}"
	DetailPrint "$(Kept)"
SectionEnd
