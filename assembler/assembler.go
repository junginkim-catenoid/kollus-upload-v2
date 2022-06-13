package assembler

import (
	"fmt"
	"io"
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

func NewFileAssembler(storagePath string) *FileAssembler {
	// Create on the heap
	return &FileAssembler{contentPath: storagePath}
}

func (fa *FileAssembler) GetSessions() []*Session {
	return fa.sessions
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

func (fa *FileAssembler) SyncAbnormalCleanupSession(upload_Key string) error {
	i, se := fa.findSession(upload_Key)

	if i >= 0 {
		fa._mutex.Lock()
		defer fa._mutex.Unlock()

		if err := se.Cleanup(); err != nil {
			return err
		}

		fa.sessions[i] = fa.sessions[len(fa.sessions)-1]
		fa.sessions[len(fa.sessions)-1] = nil
		fa.sessions = fa.sessions[:len(fa.sessions)-1]
	}

	return nil

}

func (fa *FileAssembler) GetPath() string {
	return fa.contentPath
}

func (fa *FileAssembler) CleanupSession(upload_Key string) {
	i, _ := fa.findSession(upload_Key)

	if i >= 0 {
		fa.sessions[i] = fa.sessions[len(fa.sessions)-1]
		fa.sessions[len(fa.sessions)-1] = nil
		fa.sessions = fa.sessions[:len(fa.sessions)-1]
	}
}

// findSession searches session list and returns found session
// and its index in the list.
func (fa *FileAssembler) findsession(id string) (int, *Session) {
	for i, sess := range fa.sessions {
		if sess.id == id {
			return i, sess
		}
	}
	return -1, nil
}

// Session returns a session associated with id.
// do use capitals.
func (fa *FileAssembler) GetSession(id string) *Session {
	_, sess := fa.findSession(id)
	return sess
}

/// findSession searches session list and returns found session
/// and its index in the list.
func (fa *FileAssembler) findSession(id string) (int, *Session) {
	for i, sess := range fa.sessions {
		if sess.id == id {
			return i, sess
		}
	}
	return -1, nil
}

///
/// Session CLASS
///

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

func NewSession(rootPath string) *Session {
	return &Session{rootPath: rootPath}
}

func (se *Session) GetID() string {
	return se.id
}

func (se *Session) GetOffset() int64 {
	return se.offset
}

// OffsetStr returns string representation of Offset.
func (se *Session) GetOffsetStr() string {
	return fmt.Sprintf("%d", se.offset)
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

// Put writes a file chunk to disk in a separate file.
func (se *Session) Put(chunk io.Reader) error {
	tmppath := path.Join(se.path, fmt.Sprintf("%d.tmp", se.offset))
	chunkpath := path.Join(se.path, fmt.Sprintf("%d.chunk", se.offset))

	if err := se.write(tmppath, chunk); err != nil {
		log.Println("[ERROR] write" + tmppath + "  " + chunkpath + " " + err.Error())
		return err
	}

	/// replace name to chunk
	if err := os.Rename(tmppath, chunkpath); err != nil {
		log.Println("[ERROR] Rename  " + tmppath + "  " + chunkpath + " " + err.Error())
		return err
	}

	se.chunks = append(se.chunks, chunkpath)
	return nil
}

//

func (se *Session) write(fpath string, data io.Reader) error {
	if file, err := os.Create(fpath); err != nil {
		return err
	} else {
		defer file.Close()

		if n, err := io.Copy(file, data); err != nil {
			return err
		} else {
			se.offset += int64(n)
		}
	}
	return nil
}

func (se *Session) Cleanup() error {
	return os.RemoveAll(se.path)
}

// Commit finishes an upload session by combining all its chunk into
// final destination file.
func (se *Session) Commit(filepath string) error {
	dst, err := os.OpenFile(filepath, os.O_CREATE|os.O_TRUNC|os.O_APPEND|os.O_WRONLY, 0755)

	if err != nil {
		return err
	}
	defer dst.Close()

	for _, chunk := range se.chunks {
		if file, err := os.Open(chunk); err != nil {
			return err
		} else {
			io.Copy(dst, file)
			if err := file.Close(); err != nil {
				return err
			}
		}
	}

	if err := se.Cleanup(); err != nil {
		return err
	}

	return nil
}
