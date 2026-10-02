package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// normalizePathName
// ---------------------------------------------------------------------------

func TestGetNormalizedPathName(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "spaces and mixed case",
			path: "this is a File.txt",
			want: "this-is-a-file.txt",
		},
		{
			name: "already normalized",
			path: "already-normalized.txt",
			want: "already-normalized.txt",
		},
		{
			name: "all uppercase",
			path: "MY DOCUMENT.PDF",
			want: "my-document.pdf",
		},
		{
			name: "multiple consecutive spaces",
			path: "too   many    spaces.txt",
			want: "too-many-spaces.txt",
		},
		{
			name: "leading and trailing spaces",
			path: "  padded name.txt  ",
			want: "padded-name.txt",
		},
		{
			name: "empty string",
			path: "",
			want: "",
		},
		{
			name: "no extension",
			path: "readme",
			want: "readme",
		},
		{
			name: "single character",
			path: "A",
			want: "a",
		},
		{
			name: "underscores preserved",
			path: "my_file name.txt",
			want: "my-file-name.txt",
		},
		{
			name: "multiple dots in name",
			path: "archive.tar.gz",
			want: "archive.tar.gz",
		},
		{
			name: "numbers in name",
			path: "Report 2024 Q1.xlsx",
			want: "report-2024-q1.xlsx",
		},
		{
			name: "tab and newline whitespace",
			path: "tab\tand\nnewline.txt",
			want: "tab-and-newline.txt",
		},
		{
			name: "mixed whitespace types",
			path: "mixed \t \n spaces.txt",
			want: "mixed-spaces.txt",
		},
		{
			name: "leading and trailing non-space whitespace",
			path: "\t\nfoo\t\n",
			want: "foo",
		},
		{
			name: "whitespace only spaces",
			path: "   ",
			want: "",
		},
		{
			name: "whitespace only tabs and newlines",
			path: "\t\n",
			want: "",
		},
		{
			name: "single space",
			path: " ",
			want: "",
		},
		{
			name: "hyphen between spaces produces double hyphen",
			path: "hello -world",
			want: "hello--world",
		},
		{
			name: "spaces around hyphen produce triple hyphens",
			path: "a  -  b",
			want: "a---b",
		},
		{
			name: "preserves forward slashes",
			path: "docs/2024/Q1 report.pdf",
			want: "docs/2024/q1-report.pdf",
		},
		{
			name: "unicode accented characters lowercased",
			path: "Café résumé.txt",
			want: "café-résumé.txt",
		},
		{
			name: "punctuation preserved",
			path: "My Report (v2).txt",
			want: "my-report-(v2).txt",
		},
		{
			// Regression: only the final path component may be normalized.
			// os.Rename cannot create missing intermediate directories, so
			// normalizing ancestor directory names (uppercase, spaces,
			// underscores) would produce a target path whose parent
			// directory doesn't exist on disk.
			name: "parent directories are left untouched",
			path: "My_Dir/Sub Folder/My File.txt",
			want: "My_Dir/Sub Folder/my-file.txt",
		},
		{
			name: "trailing slash treated as directory with empty basename",
			path: "My_Dir/",
			want: "My_Dir/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePathName(tt.path)
			if got != tt.want {
				t.Errorf("getNormalizedPathName(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}