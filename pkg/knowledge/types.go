package knowledge

// NoteData carries all information needed to render a ticket note.
type NoteData struct {
	Ticket       string
	TicketType   string
	Date         string
	Time         string
	Summary      string
	Status       string
	Description  string
	RepoName     string
	RepoPath     string
	WorktreePath string
}

// NoteResult contains the result of a note creation operation.
type NoteResult struct {
	Path    string
	Created bool // true if newly created, false if already existed
}
