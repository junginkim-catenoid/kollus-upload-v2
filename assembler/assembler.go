package assembler

import (
	"sync"
	"time"
)

type FileAssembler struct {
	contentPath string
	sessions    []*Session
	_mutex      sync.Mutex
}

func NewFileAssembler(storagePath string) *FileAssembler {
	// Create on the heap
	return &FileAssembler{contentPath: storagePath}
}

type Session struct {
	id string

	// Start and end time of a session. Session is
	// considered expired if now > end.
	start, end time.Time

	// chunks is a slice of chunk paths to be combined
	// into a single file
	chunks []string

	// Offset is the current offset in bytes that
	// represents the amount of data written
	offset int64

	// path is a directory where where chunks
	// for an upload session are stored
	path string
	// rootpath is a path relative to which session
	// directories are created
	rootPath string
}
