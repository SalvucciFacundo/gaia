package diff

import (
	"strings"
	"testing"
)

func TestParseUnifiedDiff_StandardMultiFile(t *testing.T) {
	rawDiff := `diff --git a/file1.go b/file1.go
index 1234567..89abcdef 100644
--- a/file1.go
+++ b/file1.go
@@ -1,4 +1,5 @@
 package main
 
-func old() {}
+func newFunc() {}
+func added() {}
 func common() {}
diff --git a/file2.go b/file2.go
new file mode 100644
--- /dev/null
+++ b/file2.go
@@ -0,0 +1,3 @@
+package main
+
+var x = 1
`

	files, err := ParseUnifiedDiff(rawDiff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}

	// Verify file 1
	f1 := files[0]
	if f1.OldPath != "file1.go" || f1.NewPath != "file1.go" {
		t.Errorf("file1 paths mismatch: old=%q, new=%q", f1.OldPath, f1.NewPath)
	}
	if f1.IsBinary {
		t.Errorf("file1 should not be binary")
	}
	if len(f1.Hunks) != 1 {
		t.Fatalf("file1 expected 1 hunk, got %d", len(f1.Hunks))
	}
	h1 := f1.Hunks[0]
	if h1.OldStart != 1 || h1.OldLines != 4 || h1.NewStart != 1 || h1.NewLines != 5 {
		t.Errorf("h1 ranges mismatch: -%d,%d +%d,%d", h1.OldStart, h1.OldLines, h1.NewStart, h1.NewLines)
	}
	if len(h1.Lines) != 6 {
		t.Fatalf("h1 expected 6 lines, got %d", len(h1.Lines))
	}
	if h1.Lines[0].Type != LineContext || h1.Lines[0].Content != "package main" || h1.Lines[0].OldLine != 1 || h1.Lines[0].NewLine != 1 {
		t.Errorf("h1 line 0 mismatch: %+v", h1.Lines[0])
	}
	if h1.Lines[1].Type != LineContext || h1.Lines[1].Content != "" || h1.Lines[1].OldLine != 2 || h1.Lines[1].NewLine != 2 {
		t.Errorf("h1 line 1 mismatch: %+v", h1.Lines[1])
	}
	if h1.Lines[2].Type != LineDeletion || h1.Lines[2].Content != "func old() {}" || h1.Lines[2].OldLine != 3 || h1.Lines[2].NewLine != 0 {
		t.Errorf("h1 line 2 mismatch: %+v", h1.Lines[2])
	}
	if h1.Lines[3].Type != LineAddition || h1.Lines[3].Content != "func newFunc() {}" || h1.Lines[3].OldLine != 0 || h1.Lines[3].NewLine != 3 {
		t.Errorf("h1 line 3 mismatch: %+v", h1.Lines[3])
	}
	if h1.Lines[4].Type != LineAddition || h1.Lines[4].Content != "func added() {}" || h1.Lines[4].OldLine != 0 || h1.Lines[4].NewLine != 4 {
		t.Errorf("h1 line 4 mismatch: %+v", h1.Lines[4])
	}
	if h1.Lines[5].Type != LineContext || h1.Lines[5].Content != "func common() {}" || h1.Lines[5].OldLine != 4 || h1.Lines[5].NewLine != 5 {
		t.Errorf("h1 line 5 mismatch: %+v", h1.Lines[5])
	}

	// Verify file 2
	f2 := files[1]
	if f2.NewPath != "file2.go" {
		t.Errorf("file2 new path mismatch: %q", f2.NewPath)
	}
	if len(f2.Hunks) != 1 {
		t.Fatalf("file2 expected 1 hunk, got %d", len(f2.Hunks))
	}
	h2 := f2.Hunks[0]
	if h2.NewStart != 1 || h2.NewLines != 3 {
		t.Errorf("h2 range mismatch: +%d,%d", h2.NewStart, h2.NewLines)
	}
}

func TestParseUnifiedDiff_Empty(t *testing.T) {
	files, err := ParseUnifiedDiff("")
	if err != nil {
		t.Fatalf("unexpected error for empty diff: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}

	files, err = ParseUnifiedDiff("   \n\n  ")
	if err != nil {
		t.Fatalf("unexpected error for whitespace diff: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestParseUnifiedDiff_BinaryFile(t *testing.T) {
	rawDiff := `diff --git a/image.png b/image.png
index 1234567..89abcdef 100644
Binary files a/image.png and b/image.png differ
`
	files, err := ParseUnifiedDiff(rawDiff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !files[0].IsBinary {
		t.Errorf("expected file to be marked as binary")
	}
	if files[0].OldPath != "image.png" || files[0].NewPath != "image.png" {
		t.Errorf("paths mismatch: old=%q, new=%q", files[0].OldPath, files[0].NewPath)
	}
	if len(files[0].Hunks) != 0 {
		t.Errorf("expected 0 hunks for binary file, got %d", len(files[0].Hunks))
	}
}

func TestDiffHunk_BuildPatch(t *testing.T) {
	rawDiff := `diff --git a/test.txt b/test.txt
index 1234567..89abcdef 100644
--- a/test.txt
+++ b/test.txt
@@ -1,3 +1,3 @@
 line 1
-line 2
+line 2 modified
 line 3
`
	files, err := ParseUnifiedDiff(rawDiff)
	if err != nil {
		t.Fatalf("ParseUnifiedDiff failed: %v", err)
	}
	if len(files) != 1 || len(files[0].Hunks) != 1 {
		t.Fatalf("expected 1 file and 1 hunk")
	}

	patch := files[0].BuildHunkPatch(0)
	if !strings.Contains(patch, "--- a/test.txt") || !strings.Contains(patch, "+++ b/test.txt") {
		t.Errorf("patch missing file headers: %s", patch)
	}
	if !strings.Contains(patch, "@@ -1,3 +1,3 @@") {
		t.Errorf("patch missing hunk header: %s", patch)
	}
	if !strings.Contains(patch, "-line 2\n+line 2 modified") {
		t.Errorf("patch missing line diffs: %s", patch)
	}
}
