package main

import (
	"testing"

	plugins "github.com/rolasotelo/goodcommit/internal/pluginruntime"
)

func TestDraftFromMessageParsesBodyAndTrailers(t *testing.T) {
	msg := "feat(core): add lock path token\n\nLine one.\nLine two.\n\nSigned-off-by: Dev <dev@example.com>\nCo-authored-by: Pair <pair@example.com>\n"
	draft := draftFromMessage(msg)

	if draft.Title != "feat(core): add lock path token" {
		t.Fatalf("title = %q", draft.Title)
	}
	if draft.Body != "Line one.\nLine two." {
		t.Fatalf("body = %q", draft.Body)
	}
	if len(draft.Trailers) != 2 {
		t.Fatalf("trailers count = %d, want 2", len(draft.Trailers))
	}
	if draft.Trailers[0].Key != "Signed-off-by" || draft.Trailers[0].Value != "Dev <dev@example.com>" {
		t.Fatalf("unexpected trailer[0] = %#v", draft.Trailers[0])
	}
	if draft.Trailers[1].Key != "Co-authored-by" || draft.Trailers[1].Value != "Pair <pair@example.com>" {
		t.Fatalf("unexpected trailer[1] = %#v", draft.Trailers[1])
	}
}

func TestRenderDraftCanonicalLayout(t *testing.T) {
	draft := plugins.CommitDraft{
		Title: "fix(cli): tighten git context errors",
		Body:  "Fails early when git metadata is missing.",
		Trailers: []plugins.Trailer{
			{Key: "Signed-off-by", Value: "Dev <dev@example.com>"},
		},
	}

	got := renderDraft(draft)
	want := "fix(cli): tighten git context errors\n\nFails early when git metadata is missing.\n\nSigned-off-by: Dev <dev@example.com>\n"
	if got != want {
		t.Fatalf("renderDraft() = %q\nwant = %q", got, want)
	}
}
