package ingest

import (
	"bytes"
	"context"
	"strings"

	"github.com/google/go-github/v66/github"
	"github.com/ledongthuc/pdf"
	"github.com/mattcheramie/repoweaver/internal/store"
)

// maxFiles caps how many static files we ingest per repo.
const maxFiles = 40

// maxFileBytes caps the size of a single fetched file.
const maxFileBytes = 512 * 1024

// fetchFiles walks the repo's default-branch tree and ingests static
// documentation: CHANGELOG*, anything under docs/, and *.pdf reference docs.
func fetchFiles(ctx context.Context, gh *github.Client, repoID int64, owner, name string) ([]store.Item, error) {
	repo, _, err := gh.Repositories.Get(ctx, owner, name)
	if err != nil {
		return nil, err
	}
	branch := repo.GetDefaultBranch()
	if branch == "" {
		branch = "main"
	}
	tree, _, err := gh.Git.GetTree(ctx, owner, name, branch, true)
	if err != nil {
		return nil, err
	}

	var items []store.Item
	count := 0
	for _, entry := range tree.Entries {
		if entry.GetType() != "blob" {
			continue
		}
		path := entry.GetPath()
		if !isDocPath(path) {
			continue
		}
		if entry.GetSize() > maxFileBytes {
			continue
		}
		text, isPDF := fetchBlobText(ctx, gh, owner, name, path)
		if strings.TrimSpace(text) == "" {
			continue
		}
		kind := store.KindDoc
		if isPDF {
			kind = store.KindPDF
		}
		items = append(items, store.Item{
			RepoID:     repoID,
			Kind:       kind,
			ExternalID: path,
			Title:      path,
			Body:       text,
			URL:        "https://github.com/" + owner + "/" + name + "/blob/" + branch + "/" + path,
			CreatedAt:  repo.GetUpdatedAt().Time,
		})
		count++
		if count >= maxFiles {
			break
		}
	}
	return items, nil
}

func isDocPath(path string) bool {
	lower := strings.ToLower(path)
	base := lower
	if i := strings.LastIndex(lower, "/"); i >= 0 {
		base = lower[i+1:]
	}
	switch {
	case strings.HasPrefix(base, "changelog"):
		return true
	case strings.HasPrefix(lower, "docs/") && (strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".txt") || strings.HasSuffix(lower, ".pdf")):
		return true
	case strings.HasSuffix(lower, ".pdf"):
		return true
	default:
		return false
	}
}

// fetchBlobText downloads a file's content, extracting text from PDFs.
func fetchBlobText(ctx context.Context, gh *github.Client, owner, name, path string) (string, bool) {
	rc, _, err := gh.Repositories.DownloadContents(ctx, owner, name, path, nil)
	if err != nil {
		return "", false
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rc); err != nil {
		return "", false
	}
	data := buf.Bytes()
	if strings.HasSuffix(strings.ToLower(path), ".pdf") {
		return extractPDF(data), true
	}
	return string(data), false
}

// extractPDF best-effort extracts plain text from a PDF byte slice. Failures
// return an empty string rather than erroring.
func extractPDF(data []byte) string {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		txt, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(txt)
		sb.WriteString("\n")
	}
	return sb.String()
}
