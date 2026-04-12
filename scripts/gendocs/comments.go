package gendocs

import (
	"regexp"
	"strings"
)

// defaultRegex matches patterns like: Default: "gzip", Defaults to "value"
var defaultRegex = regexp.MustCompile(`[Dd]efaults?(?: is|:| to)[:\s]+["']([^"']+)["']|[Dd]efaults?(?: is|:| to)[:\s]+(\$\w+)`)

// enumRegex matches patterns like: One of: json, yaml, toml.
var enumRegex = regexp.MustCompile(`[Oo]ne of:\s+(.+?)\.?\s*$`)

// CleanDoc trims and normalises a Go doc comment string.
// It preserves tab-indented lines as 4-space-indented lines so that
// markdown renderers treat them as code blocks (matching Go godoc convention).
func CleanDoc(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "\t") {
			// Preserve code block: replace leading tabs with 4 spaces each.
			trimmed := strings.TrimLeft(line, "\t")
			indent := len(line) - len(trimmed)
			lines[i] = strings.Repeat("    ", indent) + trimmed
		} else {
			lines[i] = strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}

// ExtractDefault pulls a default value from a doc comment, if present.
func ExtractDefault(doc string) string {
	if matches := defaultRegex.FindStringSubmatch(doc); len(matches) > 1 {
		val := matches[1]
		if val == "" {
			val = matches[2]
		}
		return strings.TrimSpace(val)
	}
	return ""
}

// ExtractEnum pulls an enum list from a doc comment, if present.
func ExtractEnum(doc string) []string {
	matches := enumRegex.FindStringSubmatch(doc)
	if len(matches) < 2 {
		return nil
	}
	parts := strings.Split(matches[1], ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
