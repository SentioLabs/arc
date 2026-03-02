# Content Formatting Guide

The arc frontend renders GitHub Flavored Markdown with syntax highlighting. Follow these rules when writing issue descriptions, plans, comments, and notes.

## Use
- **Fenced code blocks** with language tags: ` ```go `, ` ```bash `, ` ```json `, ` ```typescript `, ` ```sql `, ` ```yaml `, ` ```python `, ` ```html `, ` ```css `
- **Headings** (`##` and `###`) for section structure
- **Bullet lists** (`-`) for unordered items and file lists
- **Numbered lists** (`1.`) for sequential steps
- **Task lists** (`- [ ]` and `- [x]`) for checklists
- **Tables** (`| col | col |`) for structured comparisons
- **Inline code** (backticks) for file paths, function names, variable names, and CLI commands
- **Bold** (`**text**`) for emphasis on key terms
- **Blockquotes** (`>`) for important callouts or notes
- **Links** (`[text](url)`) for references

## Avoid
- Raw HTML tags — DOMPurify strips most tags
- Code fences without language tags — always specify the language for syntax highlighting
- UPPERCASE section headers (use `##` Markdown headings instead)
- Very long single-line paragraphs — use line breaks for readability

## Code Block Languages
Supported with syntax highlighting: go, typescript, javascript, json, bash, shell, sql, yaml, markdown, html, css, python, text

For unsupported languages, use `text` as the language tag.
