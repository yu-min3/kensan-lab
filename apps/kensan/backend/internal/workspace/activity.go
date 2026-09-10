package workspace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const ActivityFile = "activity.ndjson"

// Activity is an append-only observation, never inferred from a daily or Git commit.
type Activity struct {
	At      time.Time `json:"ts"`
	Kind    string    `json:"kind"`
	ID      string    `json:"id"`
	Project string    `json:"project,omitempty"`
	Text    string    `json:"text"`
	Date    string    `json:"date"`
	State   string    `json:"state,omitempty"`
	Action  string    `json:"action,omitempty"`
}

func (w *Workspace) Activities() ([]Activity, error) {
	writeMu.Lock()
	defer writeMu.Unlock()
	return w.activities()
}

func (w *Workspace) activities() ([]Activity, error) {
	f, err := os.Open(filepath.Join(w.Root, ActivityFile))
	if os.IsNotExist(err) {
		return []Activity{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := []Activity{}
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1024*1024)
	for s.Scan() {
		var e Activity
		if err := json.Unmarshal(s.Bytes(), &e); err != nil {
			return nil, fmt.Errorf("activity line %d: %w", len(out)+1, err)
		}
		out = append(out, e)
	}
	return out, s.Err()
}

// MutateEvent serializes the source edit and its observation, including retries.
// An append failure rolls the source back; a process crash between the two writes
// is not a transaction (single-process file storage, no database).
func (w *Workspace) MutateEvent(rel string, fn func([]byte, []Activity) ([]byte, *Activity, error)) error {
	abs, err := w.Abs(rel)
	if err != nil {
		return err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	content, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	events, err := w.activities()
	if err != nil {
		return err
	}
	out, event, err := fn(content, events)
	if err != nil {
		return err
	}
	if event == nil && out == nil {
		return nil
	}
	var f *os.File
	var offset int64
	var line []byte
	if event != nil {
		line, err = json.Marshal(event)
		if err != nil {
			return err
		}
		f, err = os.OpenFile(filepath.Join(w.Root, ActivityFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		offset = info.Size()
	}
	if out != nil {
		if err := os.WriteFile(abs, out, 0644); err != nil {
			return err
		}
	}
	if f != nil {
		_, err = f.Write(append(line, '\n'))
		if err == nil {
			err = f.Sync()
		}
		if err != nil {
			logErr := f.Truncate(offset)
			var restoreErr error
			if out != nil {
				restoreErr = os.WriteFile(abs, content, 0644)
			}
			return fmt.Errorf("activity write failed: %w (log rollback: %v; source rollback: %v)", err, logErr, restoreErr)
		}
	}
	return nil
}
