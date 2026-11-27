// Package viewer shows files in a browser window. A local HTTP server renders
// text files with syntax highlighting, images, and Excalidraw, Mermaid and
// draw.io diagrams. Diagram renderers are loaded from a CDN on demand.
package viewer

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

const (
	maxTextSize     = 5 << 20
	binarySniffSize = 8000
	tokenBytes      = 16
	readHeaderLimit = 5 * time.Second
)

type kind int

const (
	kindText kind = iota
	kindImage
	kindExcalidraw
	kindMermaid
	kindDrawio
	kindMarkdown
	kindHTML
	kindSVG
	kindJSON
	kindPDF
)

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".svg": true,
	".webp": true, ".bmp": true, ".ico": true, ".avif": true,
}

// ErrNotViewable is returned for binary or oversized files.
var ErrNotViewable = errors.New("file cannot be viewed")

func detectKind(path string) kind {
	ext := strings.ToLower(filepath.Ext(path))
	switch {
	case ext == ".svg":
		return kindSVG
	case imageExts[ext]:
		return kindImage
	case ext == ".excalidraw":
		return kindExcalidraw
	case ext == ".mmd" || ext == ".mermaid":
		return kindMermaid
	case ext == ".pdf":
		return kindPDF
	case ext == ".json":
		return kindJSON
	case ext == ".md" || ext == ".markdown":
		return kindMarkdown
	case ext == ".html" || ext == ".htm":
		return kindHTML
	case ext == ".drawio" || ext == ".dio":
		return kindDrawio
	default:
		return kindText
	}
}

var (
	mu      sync.Mutex
	srv     *http.Server
	baseURL string
	tokens  = map[string]entry{}
)

// Choice is an editor that can be launched from the viewer window.
type Choice struct {
	Name   string
	Launch func(path string) error
}

type entry struct {
	path    string
	editors []Choice
}

// Open serves path and opens it in a browser window. Pressing E or F4 in the
// window launches one of the editors.
func Open(path string, editors []Choice) error {
	url, err := URL(path, editors)
	if err != nil {
		return err
	}
	return launch(url)
}

// URL registers path with the local server (starting it if needed) and
// returns the address of its viewer page.
func URL(path string, editors []Choice) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%w: %s is a directory", ErrNotViewable, abs)
	}

	mu.Lock()
	defer mu.Unlock()
	if srv == nil {
		if err := startServer(); err != nil {
			return "", err
		}
	}
	buf := make([]byte, tokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	tokens[token] = entry{path: abs, editors: editors}
	return baseURL + "/v/" + token, nil
}

// startServer must be called with mu held.
func startServer() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v/", handlePage)
	mux.HandleFunc("/raw/", handleRaw)
	mux.HandleFunc("/edit/", handleEdit)
	mux.HandleFunc("/hex/", handleHex)
	srv = &http.Server{Handler: mux, ReadHeaderTimeout: readHeaderLimit}
	baseURL = "http://" + ln.Addr().String()
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("viewer server stopped", "error", err)
		}
	}()
	return nil
}

func lookup(r *http.Request, prefix string) (entry, string, bool) {
	mu.Lock()
	defer mu.Unlock()
	token := strings.TrimPrefix(r.URL.Path, prefix)
	e, ok := tokens[token]
	return e, token, ok
}

func handleRaw(w http.ResponseWriter, r *http.Request) {
	e, _, ok := lookup(r, "/raw/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	path := e.path
	if ct := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		// Prevent scripts inside an SVG from running
		w.Header().Set("Content-Security-Policy", "script-src 'none'")
	}
	http.ServeFile(w, r, path)
}

func handleEdit(w http.ResponseWriter, r *http.Request) {
	e, _, ok := lookup(r, "/edit/")
	if !ok || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	i, err := strconv.Atoi(r.URL.Query().Get("i"))
	if err != nil || i < 0 || i >= len(e.editors) {
		http.Error(w, "unknown editor", http.StatusBadRequest)
		return
	}
	if err := e.editors[i].Launch(e.path); err != nil {
		slog.Error("cannot launch editor", "editor", e.editors[i].Name, "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handlePage(w http.ResponseWriter, r *http.Request) {
	e, token, ok := lookup(r, "/v/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	body, err := RenderPage(e.path, token, e.editors)
	if err != nil {
		body = page(filepath.Base(e.path), `<p class="err">`+html.EscapeString(err.Error())+`</p>`+
			editorUI(token, e.editors))
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(body))
}

// RenderPage builds the HTML page for path. token identifies the file on the
// local server (used for raw contents and the editor endpoint).
func RenderPage(path, token string, editors []Choice) (string, error) {
	body, err := renderBody(path, "/raw/"+token)
	if err != nil {
		return "", err
	}
	return page(filepath.Base(path), body+editorUI(token, editors)), nil
}

func renderBody(path, rawURL string) (string, error) {
	name := filepath.Base(path)
	k := detectKind(path)
	if k == kindImage {
		return centered(`<img class="img" src="` + html.EscapeString(rawURL) +
			`" alt="` + html.EscapeString(name) + `">`), nil
	}

	if k == kindPDF {
		// The browser's built-in PDF viewer does the rendering
		return `<iframe id="pdf" class="frame" src="` + html.EscapeString(rawURL) + `"></iframe>` +
			`<script>(() => { const f = document.getElementById('pdf'); const focus = () => { f.focus(); try { f.contentWindow.focus(); } catch (e) {} };` +
			` f.addEventListener('load', focus); addEventListener('load', focus); focus(); })();</script>`, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sniff := make([]byte, binarySniffSize)
	n, err := io.ReadFull(f, sniff)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", err
	}
	if bytes.IndexByte(sniff[:n], 0) >= 0 {
		return hexPage(f, strings.Replace(rawURL, "/raw/", "/hex/", 1))
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxTextSize+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxTextSize {
		return "", fmt.Errorf("%w: larger than %d MB", ErrNotViewable, maxTextSize>>20) //nolint:mnd // MB shift
	}

	switch k {
	case kindExcalidraw:
		return centered(`<div id="out">Rendering...</div>`) + jsonData(string(data)) + excalidrawScript, nil
	case kindMermaid:
		return toggledVisual(`<pre class="mermaid">`+html.EscapeString(string(data))+`</pre>`+mermaidScript,
			highlight(name, string(data))), nil
	case kindSVG:
		return toggledVisual(`<img class="img" src="`+html.EscapeString(rawURL)+`" alt="`+html.EscapeString(name)+`">`,
			highlight(name, string(data))), nil
	case kindHTML:
		return toggled(sandboxFrame(string(data)), highlight(name, string(data))), nil
	case kindMarkdown:
		return toggled(`<iframe id="md" sandbox="allow-same-origin" class="frame"></iframe>`+jsonData(string(data))+markdownScript,
			highlight(name, string(data))), nil
	case kindDrawio:
		cfg, err := json.Marshal(map[string]any{
			"highlight": "#0000ff", "nav": true, "resize": true,
			"toolbar": "zoom layers lightbox", "xml": string(data),
		})
		if err != nil {
			return "", err
		}
		return centered(`<div class="mxgraph" data-mxgraph="`+html.EscapeString(string(cfg))+`"></div>`) +
			drawioScript, nil
	case kindJSON:
		raw := highlight(name, string(data))
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, data, "", "    "); err != nil {
			return raw, nil //nolint:nilerr // invalid JSON is shown as is
		}
		return toggled(highlight(name, pretty.String()), raw), nil
	case kindImage, kindText, kindPDF:
	}

	return highlight(name, string(data)), nil
}

func highlight(name, src string) string {
	lexer := lexers.Match(name)
	if lexer == nil {
		lexer = lexers.Analyse(src)
	}
	if lexer == nil {
		lexer = lexers.Fallback
	}
	lexer = chroma.Coalesce(lexer)
	style := styles.Get("github-dark")
	if style == nil {
		style = styles.Fallback
	}
	formatter := chromahtml.New(chromahtml.WithLineNumbers(true), chromahtml.TabWidth(4)) //nolint:mnd // tab width
	it, err := lexer.Tokenise(nil, src)
	if err != nil {
		return "<pre>" + html.EscapeString(src) + "</pre>"
	}
	var sb strings.Builder
	if err := formatter.Format(&sb, style, it); err != nil {
		return "<pre>" + html.EscapeString(src) + "</pre>"
	}
	return `<div class="code">` + sb.String() + `</div>`
}

// sandboxFrame shows untrusted html without allowing scripts to run
func sandboxFrame(src string) string {
	return `<iframe sandbox="allow-same-origin" class="frame" srcdoc="` + html.EscapeString(src) + `"></iframe>`
}

// centered places visual content in the middle of the window
func centered(body string) string {
	return `<div class="center">` + body + `</div>`
}

func toggledVisual(rendered, source string) string {
	return toggled(centered(rendered), source)
}

// toggled builds a page that switches between a rendered view and the
// highlighted source with Tab, v or F3.
func toggled(rendered, source string) string {
	return `<div id="rendered">` + rendered + `</div><div id="source" hidden>` + source + `</div>` + toggleScript
}

// jsonData embeds raw text so that scripts can read it without escaping issues.
func jsonData(s string) string {
	b, _ := json.Marshal(s)
	safe := strings.ReplaceAll(string(b), "<", `\u003c`)
	return `<script type="application/json" id="data">` + safe + `</script>`
}

func page(title, body string) string {
	return `<!DOCTYPE html><html><head><meta charset="utf-8"><title>` + html.EscapeString(title) + `</title>
<style>
body{margin:0;background:#0d1117;color:#c9d1d9;font-family:sans-serif}
header{padding:8px 16px;background:#161b22;border-bottom:1px solid #30363d;font-size:14px}
html,body{height:100%}
body{display:flex;flex-direction:column}
main{padding:16px;flex:1;display:flex;flex-direction:column}
main>div{flex:1}
.code pre{margin:0;padding:12px;border-radius:6px;overflow:auto;font-size:13px;line-height:1.45}
body.has-map main{padding-right:130px}
#minimap{position:fixed;right:0;bottom:0;width:110px;overflow:hidden;background:#0d1117;border-left:1px solid #30363d;cursor:pointer}
.mm-inner{position:absolute;left:0;top:0;transform-origin:0 0;pointer-events:none;opacity:.55}
.mm-inner .code pre{overflow:hidden;padding:0}
.mm-view{position:absolute;left:0;right:0;background:#ffffff22;pointer-events:none}
.img{max-width:100%;max-height:calc(100vh - 80px);display:block;margin:0 auto;background:#fff}
.mermaid,#out{background:#fff;color:#000;padding:16px;border-radius:6px;overflow:auto}
.mxgraph{background:#fff;border-radius:6px;width:100%}
.center>.mxgraph{margin:auto!important}
.frame{width:100%;height:calc(100vh - 80px);border:0;background:#fff;border-radius:6px}
.center{display:flex;height:100%;min-height:calc(100vh - 80px)}
.center>*{margin:auto}
#editors{position:fixed;inset:0;background:#0008;display:flex;align-items:center;justify-content:center}
#editors>div{background:#161b22;border:1px solid #30363d;border-radius:6px;padding:16px;min-width:240px}
#editors button{display:block;width:100%;margin:6px 0;padding:6px;background:#21262d;color:#c9d1d9;border:1px solid #30363d;border-radius:4px;cursor:pointer;text-align:left}
#editors button:focus{outline:2px solid #58a6ff}
[hidden]{display:none!important}
#search{position:fixed;bottom:0;left:0;right:0;padding:8px;background:#161b22;border-top:1px solid #30363d}
#search input{width:100%;box-sizing:border-box;padding:6px;background:#0d1117;color:#c9d1d9;border:1px solid #30363d;border-radius:4px}
.err{color:#f85149}
#hex{margin:0;font:13px/1.45 monospace;white-space:pre}
#hex .off,.hexinfo{color:#6e7681}
#hex .b0{color:#6e7681}
#hex .bp{color:#7ee787}
#hex .bw{color:#79c0ff}
#hex .bc{color:#ff7b72}
#hex .bh{color:#ffa657}
</style></head><body><header>` + html.EscapeString(title) + ` <span style="float:right;color:#8b949e">q / Esc to close</span></header><main>` + body + `</main>` + closeScript + minimapScript + `</body></html>`
}

// editorUI renders the editor picker and the E / F4 handler
func editorUI(token string, editors []Choice) string {
	if len(editors) == 0 {
		return ""
	}
	names := make([]string, len(editors))
	for i, e := range editors {
		names[i] = e.Name
	}
	list, _ := json.Marshal(names)
	return `<div id="editors" hidden><div><strong>Open with</strong></div></div>` +
		`<script>
(() => {
  const editors = ` + string(list) + `;
  const dlg = document.getElementById('editors');
  const launch = i => {
    fetch('/edit/` + token + `?i=' + i, {method: 'POST'}).then(r => {
      if (!r.ok) r.text().then(t => alert(t));
    });
    dlg.hidden = true;
  };
  const box = dlg.firstChild;
  let sel = 0;
  const select = i => {
    sel = (i + editors.length) % editors.length;
    box.querySelectorAll('button')[sel].focus();
  };
  editors.forEach((name, i) => {
    const b = document.createElement('button');
    b.textContent = (i + 1) + '. ' + name;
    b.onclick = () => launch(i);
    box.appendChild(b);
  });
  addEventListener('keydown', e => {
    if (e.target instanceof HTMLInputElement) return;
    if (!dlg.hidden) {
      e.stopImmediatePropagation();
      e.preventDefault();
      if (e.key === 'Escape') dlg.hidden = true;
      else if (e.key === 'ArrowDown' || e.key === 'j') select(sel + 1);
      else if (e.key === 'ArrowUp' || e.key === 'k') select(sel - 1);
      else if (e.key === 'Enter') launch(sel);
      else if (/^[1-9]$/.test(e.key) && editors[+e.key - 1]) launch(+e.key - 1);
      return;
    }
    if (e.key !== 'E' && e.key !== 'F4') return;
    e.preventDefault();
    if (editors.length === 1) launch(0);
    else { dlg.hidden = false; select(0); }
  }, true);
})();
</script>`
}

// minimapScript shows a scaled-down overview of highlighted code. Text is replaced
// with blocks so that it does not take part in searching.
const minimapScript = `<script>
(() => {
  const K = 0.12, W = 110, MAX_CHARS = 400000;
  const map = document.createElement('div');
  map.id = 'minimap';
  map.hidden = true;
  const inner = document.createElement('div');
  inner.className = 'mm-inner';
  const view = document.createElement('div');
  view.className = 'mm-view';
  map.append(inner, view);
  document.body.appendChild(map);
  let offset = 0, top0 = 0;
  const code = () => [...document.querySelectorAll('.code')].find(x => !map.contains(x) && x.offsetParent !== null);
  const update = () => {
    const clone = inner.firstChild;
    if (map.hidden || !clone) return;
    map.style.top = document.querySelector('header').offsetHeight + 'px';
    const src = code();
    top0 = src.getBoundingClientRect().top + scrollY;
    const maxScroll = Math.max(1, document.documentElement.scrollHeight - innerHeight);
    const scaledH = clone.offsetHeight * K, mapH = map.clientHeight;
    offset = scaledH > mapH ? Math.min(1, scrollY / maxScroll) * (scaledH - mapH) : 0;
    inner.style.transform = 'translateY(' + (-offset) + 'px) scale(' + K + ')';
    view.style.top = ((scrollY - top0) * K - offset) + 'px';
    view.style.height = innerHeight * K + 'px';
  };
  const build = () => {
    const src = code();
    map.hidden = !src || src.textContent.length > MAX_CHARS;
    document.body.classList.toggle('has-map', !map.hidden);
    if (map.hidden) return;
    const clone = src.cloneNode(true);
    clone.style.width = (W / K) + 'px';
    const walker = document.createTreeWalker(clone, NodeFilter.SHOW_TEXT);
    for (let n = walker.nextNode(); n; n = walker.nextNode()) n.nodeValue = n.nodeValue.replace(/\S/g, '\u2588');
    inner.replaceChildren(clone);
    update();
  };
  const jump = e => {
    const y = e.clientY - map.getBoundingClientRect().top;
    scrollTo(0, (y + offset) / K + top0 - innerHeight / 2);
  };
  let dragging = false;
  map.addEventListener('mousedown', e => { dragging = true; jump(e); e.preventDefault(); });
  addEventListener('mousemove', e => { if (dragging) jump(e); });
  addEventListener('mouseup', () => { dragging = false; });
  addEventListener('scroll', update);
  addEventListener('resize', update);
  addEventListener('viewchange', build);
  build();
})();
</script>`

const closeScript = `<div id="search" hidden><input placeholder="Search (Enter: next, Shift+Enter: previous, Esc: close)"></div>
<script>
(() => {
  const bar = document.getElementById('search');
  const input = bar.firstChild;
  const find = back => {
    if (!input.value) return;
    const frame = document.querySelector('iframe:not([hidden])');
    const visible = frame && frame.offsetParent !== null;
    const w = visible ? frame.contentWindow : window;
    if (!w.find(input.value, false, back, true)) input.style.outline = '2px solid #f85149';
    else input.style.outline = '';
  };
  // f fits visual content to the window, r restores its original size
  const visual = () => [...document.querySelectorAll('.center > *')].find(x => x.offsetParent !== null);
  const zoomBy = factor => {
    const el = visual();
    if (!el) return;
    el.style.maxWidth = 'none';
    el.style.maxHeight = 'none';
    el.style.zoom = Math.min(20, Math.max(0.05, (parseFloat(el.style.zoom) || 1) * factor));
  };
  const SCROLL = 80;
  const scrollKeys = {
    ArrowUp: [0, -1], k: [0, -1], ArrowDown: [0, 1], j: [0, 1],
    ArrowLeft: [-1, 0], h: [-1, 0], ArrowRight: [1, 0], l: [1, 0],
  };
  const zoomContent = fit => {
    const el = visual();
    if (!el) return;
    el.style.maxWidth = 'none';
    el.style.maxHeight = 'none';
    el.style.zoom = 1;
    if (!fit) return;
    const r = el.getBoundingClientRect();
    const aw = innerWidth - 40, ah = innerHeight - document.querySelector('header').offsetHeight - 40;
    if (r.width > 0 && r.height > 0) el.style.zoom = Math.min(aw / r.width, ah / r.height);
  };
  addEventListener('keydown', e => {
    if (e.target === input) {
      if (e.key === 'Escape') { bar.hidden = true; input.blur(); }
      else if (e.key === 'Enter') find(e.shiftKey);
      return;
    }
    // Keep the browser's own find bar from popping up (Ctrl+F, F3, quick find, find as you type)
    const printable = e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey;
    if (printable || e.key === 'F3') e.preventDefault();
    if (e.key === '/' || ((e.ctrlKey || e.metaKey) && e.key === 'f')) {
      e.preventDefault();
      bar.hidden = false;
      input.focus();
      input.select();
      return;
    }
    if (e.key === '+' || e.key === '=') { zoomBy(1.25); return; }
    if (e.key === '-' || e.key === '_') { zoomBy(0.8); return; }
    if (scrollKeys[e.key]) {
      e.preventDefault();
      const [dx, dy] = scrollKeys[e.key];
      scrollBy(dx * SCROLL, dy * SCROLL);
      return;
    }
    if (e.key === 'f' || e.key === 'r') { zoomContent(e.key === 'f'); return; }
    if (e.key === 'q' || e.key === 'Escape') window.close();
  });
})();
</script>`

const toggleScript = `<script>
addEventListener('keydown', e => {
  if (e.target instanceof HTMLInputElement) return;
  if (e.key !== 'Tab' && e.key !== 'v' && e.key !== 'F3') return;
  e.preventDefault();
  const r = document.getElementById('rendered'), s = document.getElementById('source');
  r.hidden = !r.hidden;
  s.hidden = !s.hidden;
  dispatchEvent(new Event('viewchange'));
});
</script>`

const markdownScript = `<script type="module">
const frame = document.getElementById('md');
const text = JSON.parse(document.getElementById('data').textContent);
try {
  const { marked } = await import('https://cdn.jsdelivr.net/npm/marked@12/lib/marked.esm.js');
  const css = 'body{font-family:sans-serif;max-width:860px;margin:16px auto;padding:0 16px;line-height:1.5}' +
    'pre,code{background:#f3f3f3}pre{padding:8px;overflow:auto}img{max-width:100%}' +
    'table{border-collapse:collapse}td,th{border:1px solid #ccc;padding:4px 8px}';
  frame.srcdoc = '<style>' + css + '</style>' + marked.parse(text);
} catch (e) {
  frame.srcdoc = '<pre>Cannot render Markdown: ' + String(e).replace(/</g, '&lt;') + '</pre>';
}
</script>`

const mermaidScript = `<script type="module">
import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs';
mermaid.initialize({startOnLoad: true});
</script>`

const drawioScript = `<script src="https://viewer.diagrams.net/js/viewer-static.min.js"></script>`

const excalidrawScript = `<script type="module">
const out = document.getElementById('out');
try {
  const mod = await import('https://esm.sh/@excalidraw/excalidraw@0.17.6?deps=react@18.2.0,react-dom@18.2.0');
  const exportToSvg = mod.exportToSvg || (mod.default && mod.default.exportToSvg);
  if (!exportToSvg) throw new Error('exportToSvg not found in ' + Object.keys(mod).join(','));
  const scene = JSON.parse(JSON.parse(document.getElementById('data').textContent));
  const svg = await exportToSvg({
    elements: scene.elements || [],
    appState: { ...(scene.appState || {}), exportBackground: true, viewBackgroundColor: '#ffffff' },
    files: scene.files || {},
  });
  svg.style.maxWidth = '100%';
  svg.style.height = 'auto';
  out.replaceChildren(svg);
} catch (e) {
  out.textContent = 'Cannot render Excalidraw file: ' + e;
}
</script>`

// launch opens url in an app-style browser window when one is available,
// falling back to the system default opener.
func launch(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		for _, b := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome"} {
			if p, err := exec.LookPath(b); err == nil {
				cmd = exec.Command(p, "--app="+url) //nolint:gosec // fixed browser names, local URL
				break
			}
		}
		if cmd == nil {
			cmd = exec.Command("xdg-open", url)
		}
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot open viewer: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
