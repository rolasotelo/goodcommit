package main

import (
	"regexp"
	"strings"

	plugins "github.com/rolasotelo/goodcommit/internal/pluginruntime"
)

func renderDraft(d plugins.CommitDraft) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(d.Title))
	if d.Body != "" {
		b.WriteString("\n\n")
		b.WriteString(strings.TrimRight(d.Body, "\n"))
	}
	if len(d.Trailers) > 0 {
		b.WriteString("\n\n")
		for i, t := range d.Trailers {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(strings.TrimSpace(t.Key))
			b.WriteString(": ")
			b.WriteString(strings.TrimSpace(t.Value))
		}
	}
	b.WriteByte('\n')
	return b.String()
}

var trailerLineRe = regexp.MustCompile(`^[A-Za-z0-9-]+:\s+.+$`)

func draftFromMessage(message string) plugins.CommitDraft {
	msg := strings.ReplaceAll(message, "\r\n", "\n")
	msg = strings.TrimRight(msg, "\n")
	parts := strings.SplitN(msg, "\n", 2)
	title := ""
	rest := ""
	if len(parts) > 0 {
		title = parts[0]
	}
	if len(parts) == 2 {
		rest = strings.TrimLeft(parts[1], "\n")
	}

	body := rest
	trailers := []plugins.Trailer{}
	if rest != "" {
		lines := strings.Split(rest, "\n")
		i := len(lines) - 1
		for i >= 0 && strings.TrimSpace(lines[i]) == "" {
			i--
		}
		start := i
		for start >= 0 && trailerLineRe.MatchString(lines[start]) {
			start--
		}
		start++
		if start <= i && start >= 0 {
			for _, tl := range lines[start : i+1] {
				kv := strings.SplitN(tl, ":", 2)
				if len(kv) == 2 {
					trailers = append(trailers, plugins.Trailer{Key: strings.TrimSpace(kv[0]), Value: strings.TrimSpace(kv[1])})
				}
			}
			body = strings.TrimRight(strings.Join(lines[:start], "\n"), "\n")
		}
	}

	return plugins.CommitDraft{Title: title, Body: body, Trailers: trailers, Metadata: map[string]interface{}{}}
}
