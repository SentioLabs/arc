package docsearch

import (
	"regexp"
	"strings"

	"github.com/sentiolabs/arc/internal/docs"
)

// Preview length constants for makePreview.
const (
	maxPreviewLength      = 200 // maximum content length for preview
	wordBoundaryThreshold = 100 // minimum index for word boundary truncation
)

// DocChunk represents a searchable section of documentation.
type DocChunk struct {
	ID      string // e.g., "workflows/session-start"
	Topic   string // e.g., "workflows"
	Heading string // e.g., "Session Start Checklist"
	Content string // full text for indexing
	Preview string // first ~200 chars for display
}

// topicDocs maps topic names to their embedded content.
var topicDocs = map[string]string{
	"workflows":    docs.Workflows,
	"dependencies": docs.Dependencies,
	"boundaries":   docs.Boundaries,
	"resumability": docs.Resumability,
	"plugin":       docs.Plugin,
}

// ChunkAllDocs splits all embedded documentation into searchable chunks.
func ChunkAllDocs() []DocChunk {
	chunks := make([]DocChunk, 0, len(topicDocs)+1)

	// Add overview as a single chunk
	chunks = append(chunks, DocChunk{
		ID:      "overview",
		Topic:   "overview",
		Heading: "Arc Documentation Overview",
		Content: docs.Overview,
		Preview: makePreview(docs.Overview),
	})

	// Chunk each topic's markdown
	for topic, content := range topicDocs {
		chunks = append(chunks, ChunkMarkdown(topic, content)...)
	}

	return chunks
}

// headerPattern matches markdown headers (## or ###) with optional anchor {#id}
var headerPattern = regexp.MustCompile(`^(#{2,3})\s+(.+?)(?:\s*\{#([^}]+)\})?\s*$`)

// ChunkMarkdown splits a markdown document into chunks by headers.
func ChunkMarkdown(topic, content string) []DocChunk {
	var chunks []DocChunk
	lines := strings.Split(content, "\n")

	var currentChunk *DocChunk
	var contentLines []string

	// Helper to finalize current chunk
	finalizeChunk := func() {
		if currentChunk != nil {
			currentChunk.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
			currentChunk.Preview = makePreview(currentChunk.Content)
			if currentChunk.Content != "" {
				chunks = append(chunks, *currentChunk)
			}
		}
	}

	for _, line := range lines {
		if matches := headerPattern.FindStringSubmatch(line); matches != nil {
			// Finalize previous chunk
			finalizeChunk()

			// Start new chunk
			heading := strings.TrimSpace(matches[2])
			id := matches[3]
			if id == "" {
				id = slugify(heading)
			}

			currentChunk = &DocChunk{
				ID:      topic + "/" + id,
				Topic:   topic,
				Heading: heading,
			}
			contentLines = []string{heading}
		} else if currentChunk != nil {
			contentLines = append(contentLines, line)
		}
	}

	// Finalize last chunk
	finalizeChunk()

	return chunks
}

// makePreview creates a preview of the content (first ~200 chars).
func makePreview(content string) string {
	// Remove markdown code blocks for cleaner preview
	content = regexp.MustCompile("(?s)```.*?```").ReplaceAllString(content, "")

	// Remove markdown headers
	content = regexp.MustCompile(`(?m)^#{1,6}\s+`).ReplaceAllString(content, "")

	// Collapse whitespace
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	content = strings.TrimSpace(content)

	if len(content) > maxPreviewLength {
		// Find a good break point
		content = content[:maxPreviewLength]
		if idx := strings.LastIndex(content, " "); idx > wordBoundaryThreshold {
			content = content[:idx]
		}
		content += "..."
	}

	return content
}

// slugify converts a heading to a URL-safe ID.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`[^a-z0-9\s-]`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
