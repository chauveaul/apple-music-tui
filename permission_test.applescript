try
    tell application "Music"
        activate
        delay 1
        get name of current track
    end tell
on error errMsg
    display dialog "Error: " & errMsg
end try
