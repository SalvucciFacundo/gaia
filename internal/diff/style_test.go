package diff

import (
	"strings"
	"testing"
)

func TestRenderDiffFile(t *testing.T) {
	file := DiffFile{
		OldPath: "example.go",
		NewPath: "example.go",
		Hunks: []DiffHunk{
			{
				Header:   "@@ -1,3 +1,3 @@",
				OldStart: 1,
				OldLines: 3,
				NewStart: 1,
				NewLines: 3,
				Lines: []DiffLine{
					{Type: LineContext, Content: "package main", OldLine: 1, NewLine: 1},
					{Type: LineDeletion, Content: "func old()", OldLine: 2, NewLine: 0},
					{Type: LineAddition, Content: "func new()", OldLine: 0, NewLine: 2},
				},
			},
		},
	}

	rendered := RenderDiffFile(file, 80, 0)
	if !strings.Contains(rendered, "example.go") {
		t.Errorf("rendered file should contain path: %s", rendered)
	}
	if !strings.Contains(rendered, "@@ -1,3 +1,3 @@") {
		t.Errorf("rendered file should contain hunk header: %s", rendered)
	}
	if !strings.Contains(rendered, "package main") {
		t.Errorf("rendered file should contain context content: %s", rendered)
	}
	if !strings.Contains(rendered, "func old()") {
		t.Errorf("rendered file should contain deletion content: %s", rendered)
	}
	if !strings.Contains(rendered, "func new()") {
		t.Errorf("rendered file should contain addition content: %s", rendered)
	}
}

func TestRenderLineStyles(t *testing.T) {
	additionLine := DiffLine{Type: LineAddition, Content: "added line", OldLine: 0, NewLine: 10}
	renderedAdd := RenderLine(additionLine, 80)
	if !strings.Contains(renderedAdd, "added line") || !strings.Contains(renderedAdd, "+") {
		t.Errorf("expected addition line formatting, got: %s", renderedAdd)
	}

	deletionLine := DiffLine{Type: LineDeletion, Content: "deleted line", OldLine: 5, NewLine: 0}
	renderedDel := RenderLine(deletionLine, 80)
	if !strings.Contains(renderedDel, "deleted line") || !strings.Contains(renderedDel, "-") {
		t.Errorf("expected deletion line formatting, got: %s", renderedDel)
	}

	contextLine := DiffLine{Type: LineContext, Content: "context line", OldLine: 1, NewLine: 1}
	renderedCtx := RenderLine(contextLine, 80)
	if !strings.Contains(renderedCtx, "context line") {
		t.Errorf("expected context line formatting, got: %s", renderedCtx)
	}
}

func TestRenderDiffFile_Binary(t *testing.T) {
	file := DiffFile{
		OldPath:  "image.png",
		NewPath:  "image.png",
		IsBinary: true,
	}

	rendered := RenderDiffFile(file, 80, -1)
	if !strings.Contains(rendered, "Binary file") || !strings.Contains(rendered, "image.png") {
		t.Errorf("expected binary file notification, got: %s", rendered)
	}
}
