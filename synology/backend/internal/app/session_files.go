package app

// FileStationSession selects the interactive File Station adapter using the caller’s DSM session.
const FileStationSession = "session"

// SessionFilesEnabled accepts only the fixed build-time session setting.
func SessionFilesEnabled(fileStation string) bool {
	return fileStation == FileStationSession
}
