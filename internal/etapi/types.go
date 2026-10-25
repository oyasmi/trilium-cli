package etapi

import "fmt"

// The types below mirror the schemas of Trilium's ETAPI OpenAPI spec
// (GET /etapi/etapi.openapi.yaml). Field names follow the spec so that
// --json output can pass entities through unchanged.

// Note is a note as returned by ETAPI.
type Note struct {
	NoteID          string      `json:"noteId"`
	IsProtected     bool        `json:"isProtected"`
	Title           string      `json:"title"`
	Type            string      `json:"type"`
	Mime            string      `json:"mime"`
	BlobID          string      `json:"blobId,omitempty"`
	DateCreated     string      `json:"dateCreated,omitempty"`
	DateModified    string      `json:"dateModified,omitempty"`
	UTCDateCreated  string      `json:"utcDateCreated,omitempty"`
	UTCDateModified string      `json:"utcDateModified,omitempty"`
	ParentNoteIDs   []string    `json:"parentNoteIds"`
	ChildNoteIDs    []string    `json:"childNoteIds"`
	ParentBranchIDs []string    `json:"parentBranchIds"`
	ChildBranchIDs  []string    `json:"childBranchIds"`
	Attributes      []Attribute `json:"attributes"`
}

// CreateNoteDef is the request body of POST /create-note.
// Content is always sent: the server answers 500 when it is missing.
type CreateNoteDef struct {
	ParentNoteID string `json:"parentNoteId"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	Mime         string `json:"mime,omitempty"`
	Content      string `json:"content"`
	NotePosition *int   `json:"notePosition,omitempty"`
	Prefix       string `json:"prefix,omitempty"`
	IsExpanded   *bool  `json:"isExpanded,omitempty"`
	NoteID       string `json:"noteId,omitempty"`
	BranchID     string `json:"branchId,omitempty"`
}

// NotePatch is the request body of PATCH /notes/{id}. Only set fields are sent.
type NotePatch struct {
	Title           *string `json:"title,omitempty"`
	Type            *string `json:"type,omitempty"`
	Mime            *string `json:"mime,omitempty"`
	DateCreated     *string `json:"dateCreated,omitempty"`
	UTCDateCreated  *string `json:"utcDateCreated,omitempty"`
	UTCDateModified *string `json:"utcDateModified,omitempty"`
}

// NoteWithBranch is returned by create-note and import.
type NoteWithBranch struct {
	Note   Note   `json:"note"`
	Branch Branch `json:"branch"`
}

// Branch places a note into the tree.
type Branch struct {
	BranchID        string `json:"branchId"`
	NoteID          string `json:"noteId"`
	ParentNoteID    string `json:"parentNoteId"`
	Prefix          string `json:"prefix"`
	NotePosition    int    `json:"notePosition"`
	IsExpanded      bool   `json:"isExpanded"`
	UTCDateModified string `json:"utcDateModified,omitempty"`
}

// BranchCreate is the request body of POST /branches.
type BranchCreate struct {
	NoteID       string  `json:"noteId"`
	ParentNoteID string  `json:"parentNoteId"`
	Prefix       *string `json:"prefix,omitempty"`
	NotePosition *int    `json:"notePosition,omitempty"`
	IsExpanded   *bool   `json:"isExpanded,omitempty"`
}

// BranchPatch is the request body of PATCH /branches/{id}.
type BranchPatch struct {
	Prefix       *string `json:"prefix,omitempty"`
	NotePosition *int    `json:"notePosition,omitempty"`
	IsExpanded   *bool   `json:"isExpanded,omitempty"`
}

// Attribute is a label or a relation owned by a note.
type Attribute struct {
	AttributeID     string `json:"attributeId"`
	NoteID          string `json:"noteId"`
	Type            string `json:"type"`
	Name            string `json:"name"`
	Value           string `json:"value"`
	Position        int    `json:"position"`
	IsInheritable   bool   `json:"isInheritable"`
	UTCDateModified string `json:"utcDateModified,omitempty"`
}

// AttributeCreate is the request body of POST /attributes.
type AttributeCreate struct {
	NoteID        string `json:"noteId"`
	Type          string `json:"type"`
	Name          string `json:"name"`
	Value         string `json:"value"`
	IsInheritable bool   `json:"isInheritable"`
	Position      *int   `json:"position,omitempty"`
}

// AttributePatch is the request body of PATCH /attributes/{id}.
type AttributePatch struct {
	Value    *string `json:"value,omitempty"`
	Position *int    `json:"position,omitempty"`
}

// Attachment is content owned by a note.
type Attachment struct {
	AttachmentID                    string `json:"attachmentId"`
	OwnerID                         string `json:"ownerId"`
	Role                            string `json:"role"`
	Mime                            string `json:"mime"`
	Title                           string `json:"title"`
	Position                        int    `json:"position"`
	BlobID                          string `json:"blobId,omitempty"`
	DateModified                    string `json:"dateModified,omitempty"`
	UTCDateModified                 string `json:"utcDateModified,omitempty"`
	UTCDateScheduledForErasureSince string `json:"utcDateScheduledForErasureSince,omitempty"`
	ContentLength                   int    `json:"contentLength"`
}

// AttachmentCreate is the request body of POST /attachments.
type AttachmentCreate struct {
	OwnerID  string `json:"ownerId"`
	Role     string `json:"role"`
	Mime     string `json:"mime"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Position *int   `json:"position,omitempty"`
}

// AttachmentPatch is the request body of PATCH /attachments/{id}.
type AttachmentPatch struct {
	Role     *string `json:"role,omitempty"`
	Mime     *string `json:"mime,omitempty"`
	Title    *string `json:"title,omitempty"`
	Position *int    `json:"position,omitempty"`
}

// Revision is a snapshot of a note.
type Revision struct {
	RevisionID        string `json:"revisionId"`
	NoteID            string `json:"noteId"`
	Type              string `json:"type"`
	Mime              string `json:"mime"`
	IsProtected       bool   `json:"isProtected"`
	Title             string `json:"title"`
	BlobID            string `json:"blobId,omitempty"`
	DateLastEdited    string `json:"dateLastEdited,omitempty"`
	DateCreated       string `json:"dateCreated,omitempty"`
	UTCDateLastEdited string `json:"utcDateLastEdited,omitempty"`
	UTCDateCreated    string `json:"utcDateCreated,omitempty"`
	UTCDateModified   string `json:"utcDateModified,omitempty"`
	ContentLength     int    `json:"contentLength"`
}

// RecentChange is one entry of GET /notes/history.
type RecentChange struct {
	NoteID             string `json:"noteId"`
	Title              string `json:"title"`
	CurrentTitle       string `json:"current_title"`
	CurrentIsDeleted   Bool   `json:"current_isDeleted"`
	CurrentDeleteID    string `json:"current_deleteId,omitempty"`
	CurrentIsProtected Bool   `json:"current_isProtected"`
	UTCDate            string `json:"utcDate"`
	Date               string `json:"date"`
	CanBeUndeleted     Bool   `json:"canBeUndeleted,omitempty"`
}

// Bool accepts JSON booleans as well as the 0/1 integers some endpoints
// (e.g. /notes/history) return straight from SQLite.
type Bool bool

// UnmarshalJSON implements json.Unmarshaler.
func (b *Bool) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case "true", "1":
		*b = true
	case "false", "0", "null":
		*b = false
	default:
		return fmt.Errorf("invalid boolean %s", data)
	}
	return nil
}

// AppInfo is returned by GET /app-info.
type AppInfo struct {
	AppVersion             string `json:"appVersion"`
	DBVersion              int    `json:"dbVersion"`
	SyncVersion            int    `json:"syncVersion"`
	BuildDate              string `json:"buildDate"`
	BuildRevision          string `json:"buildRevision"`
	DataDirectory          string `json:"dataDirectory"`
	ClipperProtocolVersion string `json:"clipperProtocolVersion"`
	UTCDateTime            string `json:"utcDateTime"`
}

// Metrics is returned by GET /metrics?format=json.
type Metrics struct {
	Version struct {
		App           string `json:"app"`
		DB            int    `json:"db"`
		Node          string `json:"node"`
		Sync          int    `json:"sync"`
		BuildDate     string `json:"buildDate"`
		BuildRevision string `json:"buildRevision"`
	} `json:"version"`
	Database struct {
		TotalNotes         int `json:"totalNotes"`
		DeletedNotes       int `json:"deletedNotes"`
		ActiveNotes        int `json:"activeNotes"`
		ProtectedNotes     int `json:"protectedNotes"`
		TotalAttachments   int `json:"totalAttachments"`
		DeletedAttachments int `json:"deletedAttachments"`
		ActiveAttachments  int `json:"activeAttachments"`
		TotalRevisions     int `json:"totalRevisions"`
		TotalBranches      int `json:"totalBranches"`
		TotalAttributes    int `json:"totalAttributes"`
		TotalBlobs         int `json:"totalBlobs"`
		TotalEtapiTokens   int `json:"totalEtapiTokens"`
		TotalRecentNotes   int `json:"totalRecentNotes"`
	} `json:"database"`
	NoteTypes       map[string]int `json:"noteTypes"`
	AttachmentTypes map[string]int `json:"attachmentTypes"`
	Statistics      struct {
		OldestNote        *string `json:"oldestNote"`
		NewestNote        *string `json:"newestNote"`
		LastModified      *string `json:"lastModified"`
		DatabaseSizeBytes *int64  `json:"databaseSizeBytes"`
	} `json:"statistics"`
	Timestamp string `json:"timestamp"`
}

// SearchParams are the query parameters of GET /notes.
type SearchParams struct {
	Search               string
	FastSearch           bool
	IncludeArchivedNotes bool
	AncestorNoteID       string
	AncestorDepth        string
	OrderBy              string
	OrderDirection       string
	Limit                int // 0 = no limit parameter
}

// SearchResponse is returned by GET /notes.
type SearchResponse struct {
	Results []Note `json:"results"`
}
