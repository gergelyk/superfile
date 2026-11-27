package viewer

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestRenderPage(t *testing.T) {
	tests := []struct {
		name, file, content, want string
		wantErr                   bool
	}{
		{"text", "a.go", "package main\nfunc <b>() {}", `class="code"`, false},
		{"image", "a.png", "x", `src="/raw/t"`, false},
		{"mermaid", "a.mmd", "graph TD; A-->B", `class="mermaid"`, false},
		{"drawio", "a.drawio", "<mxfile></mxfile>", "data-mxgraph", false},
		{"excalidraw", "a.excalidraw", `{"elements":[]}`, "exportToSvg", false},
		{"svg", "a.svg", "<svg></svg>", `id="source" hidden`, false},
		{"markdown", "a.md", "# hi", "marked", false},
		{"html", "a.html", "<p>hi</p>", `<iframe sandbox`, false},
		{"json", "a.json", `{"a":1}`, "    ", false},
		{"editors", "a.txt", "x", "/edit/t", false},
		{"pdf", "a.pdf", "%PDF-1.4\x00", `<iframe id="pdf" class="frame" src="/raw/t"`, false},
		{"binary", "a.bin", "a\x00b", `<span class="off">00000000</span>`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RenderPage(write(t, tt.file, tt.content), "t", []Choice{{Name: "code"}})
			if tt.wantErr {
				require.ErrorIs(t, err, ErrNotViewable)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, out, tt.want)
			if tt.name == "text" {
				assert.NotContains(t, out, "<b>")
			}
		})
	}
}

func TestServer(t *testing.T) {
	url, err := URL(write(t, "a.txt", "hello"), nil)
	require.NoError(t, err)
	resp, err := http.Get(url) //nolint:gosec,noctx // local test server
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	bad, err := http.Get(strings.Replace(url, "/v/", "/v/nope", 1)) //nolint:gosec,noctx // local test server
	require.NoError(t, err)
	defer bad.Body.Close()
	assert.Equal(t, http.StatusNotFound, bad.StatusCode)
}
