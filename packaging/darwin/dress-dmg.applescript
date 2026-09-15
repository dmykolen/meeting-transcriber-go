-- Lay out the disk image window: the background, where the two icons sit, and
-- no toolbar or sidebar to distract from the one thing to do.
--
-- Finder is the only thing on macOS that can set these, and it does it by
-- writing a .DS_Store into the image — which is why the image has to be
-- mounted read/write while this runs.
on run argv
	set volumeName to item 1 of argv
	-- Fail fast and quietly if Finder is not answering at all, rather than
	-- holding the build for the length of every timeout below.
	with timeout of 8 seconds
		tell application "Finder" to get name of startup disk
	end timeout
	tell application "Finder"
		-- A timeout, because a Finder that is busy or waiting on an automation
		-- permission will otherwise hang the build for two minutes and then
		-- fail anyway.
		with timeout of 45 seconds
		tell disk volumeName
			open
			set current view of container window to icon view
			set toolbar visible of container window to false
			set statusbar visible of container window to false
			set the bounds of container window to {200, 140, 860, 540}
			set opts to the icon view options of container window
			set arrangement of opts to not arranged
			set icon size of opts to 128
			set background picture of opts to file ".background:background.tiff"
			set position of item "Meeting Transcriber.app" of container window to {180, 215}
			set position of item "Applications" of container window to {480, 215}
			close
			open
			update without registering applications
			delay 1
			close
		end tell
		end timeout
	end tell
end run
