package assembler

import (
	"log"
	"os"
	"path"
	"sync"
	"time"
)

type FileAssembler struct {
	contentPath string
	sessions    []*Session
	_mutex      sync.Mutex
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

func NewFileAssembler(storagePath string) *FileAssembler {
	// Create on the heap
	return &FileAssembler{contentPath: storagePath}
}

func NewSession(rootPath string) *Session {
	return &Session{rootPath: rootPath}
}

/// Creates session after constructor of the NewFileAssembler
func (fa *FileAssembler) CreateSession(upload_Key string) (*Session, error) {

	obj := NewSession(fa.contentPath)

	{
		fa._mutex.Lock()
		defer fa._mutex.Unlock()
		if err := obj.initSession(upload_Key); err != nil {
			return nil, err
		}
		fa.sessions = append(fa.sessions, obj)
	}
	return obj, nil
}

func (se *Session) initSession(upload_Key string) error {
	now := time.Now()
	//se.id = fmt.Sprintf("%d", now.UnixNano())

	se.id = upload_Key
	se.start = now
	se.end = now.AddDate(0, 0, 1)
	se.path = path.Join(se.rootPath, se.id)

	if err := os.Mkdir(se.path, 0775); err != nil {
		log.Println("[ERROR]Creates directory was failed ," + se.path)
		return err
	}
	return nil
}

func (fa *FileAssembler) CleanupSession(upload_Key string) {
	i, _ := fa.findSession(upload_Key)

	if i >= 0 {
		fa.sessions[i] = fa.sessions[len(fa.sessions)-1]
		fa.sessions[len(fa.sessions)-1] = nil
		fa.sessions = fa.sessions[:len(fa.sessions)-1]
	}
}

func (fa *FileAssembler) findSession(id string) (int, *Session) {
	for i, sess := range fa.sessions {
		if sess.id == id {
			return i, sess
		}
	}
	return -1, nil
}
