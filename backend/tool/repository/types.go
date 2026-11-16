package repository

type TreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int64  `json:"size,omitempty"`
}

type TreePage struct {
	Items      []TreeEntry `json:"items"`
	Offset     int         `json:"offset"`
	NextOffset int         `json:"next_offset"`
	Total      int         `json:"total"`
	Done       bool        `json:"done"`
}

type SearchPage struct {
	Items           []map[string]any `json:"items"`
	Offset          int              `json:"offset"`
	NextOffset      int              `json:"next_offset"`
	ObservedMatches int              `json:"observed_matches"`
	Done            bool             `json:"done"`
	Complete        bool             `json:"complete"`
	Truncated       bool             `json:"truncated"`
}

type SearchOptions struct {
	Pattern           string
	Path              string
	Offset            int
	Limit             int
	Revision          string
	RegularExpression bool
	IgnoreCase        bool
}

type FileRange struct {
	Path       string `json:"path"`
	Content    string `json:"content"`
	Offset     int64  `json:"offset"`
	NextOffset int64  `json:"next_offset"`
	TotalBytes int64  `json:"total_bytes"`
	Done       bool   `json:"done"`
}
