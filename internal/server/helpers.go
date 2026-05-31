package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mattcheramie/repoweaver/internal/store"
)

// parseRepoInput accepts "owner/name" or a full GitHub URL and returns the
// owner and repository name.
func parseRepoInput(in string) (owner, name string, ok bool) {
	s := strings.TrimSpace(in)
	s = strings.TrimPrefix(s, "https://github.com/")
	s = strings.TrimPrefix(s, "http://github.com/")
	s = strings.TrimSuffix(s, ".git")
	s = strings.Trim(s, "/")
	parts := strings.Split(s, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// pathID parses the {id} path value as an int64.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

// repoFromPath loads the repo referenced by the {id} path value.
func (s *Server) repoFromPath(w http.ResponseWriter, r *http.Request) (store.Repo, bool) {
	id, ok := pathID(w, r)
	if !ok {
		return store.Repo{}, false
	}
	repo, err := s.store.RepoByID(id)
	if err != nil {
		http.NotFound(w, r)
		return store.Repo{}, false
	}
	return repo, true
}

// slugify converts a title into a filesystem-safe slug for downloads.
func slugify(title string) string {
	slug := strings.ToLower(strings.TrimSpace(title))
	var b strings.Builder
	prevDash := false
	for _, r := range slug {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "content"
	}
	return out
}
