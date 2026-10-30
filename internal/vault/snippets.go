package vault

import (
	"errors"
	"sort"
	"strings"
)

// Snippet is a saved command that can be run on several hosts at once.
type Snippet struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Command string `json:"command"`
}

func (s *Store) Snippets() []Snippet {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Snippet{}, s.data.Snippets...)
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

func (s *Store) SaveSnippet(sn Snippet) (Snippet, error) {
	sn.Name = strings.TrimSpace(sn.Name)
	sn.Command = strings.TrimSpace(sn.Command)
	if sn.Command == "" {
		return Snippet{}, errors.New("perintah wajib diisi")
	}
	if sn.Name == "" {
		sn.Name = strings.SplitN(sn.Command, "\n", 2)[0]
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i, existing := range s.data.Snippets {
		if existing.ID == sn.ID {
			s.data.Snippets[i] = sn
			return sn, s.flush()
		}
	}
	sn.ID = newID()
	s.data.Snippets = append(s.data.Snippets, sn)
	return sn, s.flush()
}

func (s *Store) DeleteSnippet(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sn := range s.data.Snippets {
		if sn.ID == id {
			s.data.Snippets = append(s.data.Snippets[:i], s.data.Snippets[i+1:]...)
			return s.flush()
		}
	}
	return nil
}

func (s *Store) hasSnippetLocked(sn Snippet) bool {
	for _, e := range s.data.Snippets {
		if e.Name == sn.Name && e.Command == sn.Command {
			return true
		}
	}
	return false
}
