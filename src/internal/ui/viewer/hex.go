package viewer

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const (
	hexWidth = 16
	// hexChunk is the amount of a binary file loaded at a time
	hexChunk = 16 << 10
)

// byteClass picks the colour class of a byte: null, printable, whitespace,
// control and high (non-ASCII) bytes each get their own.
func byteClass(b byte) string {
	switch {
	case b == 0:
		return "b0"
	case b == ' ' || b == '\t' || b == '\n' || b == '\r':
		return "bw"
	case b > '~':
		return "bh"
	case b < ' ' || b == 0x7f:
		return "bc"
	default:
		return "bp"
	}
}

// hexRows renders data (starting at the 16-byte aligned offset off) as
// coloured offset, hex and ASCII columns.
func hexRows(data []byte, off int64) string {
	var sb strings.Builder
	for i := 0; i < len(data); i += hexWidth {
		line := data[i:min(i+hexWidth, len(data))]
		fmt.Fprintf(&sb, `<span class="off">%08x</span>  `, off+int64(i))
		for j := range hexWidth {
			if j < len(line) {
				fmt.Fprintf(&sb, `<span class="%s">%s</span> `, byteClass(line[j]), hex.EncodeToString(line[j:j+1]))
			} else {
				sb.WriteString("   ")
			}
			if j == hexWidth/2-1 {
				sb.WriteByte(' ')
			}
		}
		sb.WriteString(" ")
		for _, b := range line {
			c := "."
			if b >= ' ' && b <= '~' {
				c = html.EscapeString(string(rune(b)))
			}
			fmt.Fprintf(&sb, `<span class="%s">%s</span>`, byteClass(b), c)
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// readChunk reads up to hexChunk bytes at off and returns the next offset, or
// -1 at the end of the file.
func readChunk(f io.ReaderAt, size, off int64) ([]byte, int64, error) {
	buf := make([]byte, min(hexChunk, size-off))
	n, err := f.ReadAt(buf, off)
	if err != nil && err != io.EOF {
		return nil, 0, err
	}
	next := off + int64(n)
	if next >= size {
		next = -1
	}
	return buf[:n], next, nil
}

// hexPage shows the beginning of a binary file; the rest is fetched in
// fragments from hexURL while scrolling.
func hexPage(f *os.File, hexURL string) (string, error) {
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	data, next, err := readChunk(f, info.Size(), 0)
	if err != nil {
		return "", err
	}
	urlJSON, _ := json.Marshal(hexURL)
	return `<div class="hexinfo">` + strconv.FormatInt(info.Size(), 10) + ` bytes (loaded in fragments while scrolling)</div>` +
		`<pre id="hex">` + hexRows(data, 0) + `</pre>` + `<script>
(() => {
  const pre = document.getElementById('hex');
  const url = ` + string(urlJSON) + `;
  let next = ` + strconv.FormatInt(next, 10) + `, busy = false;
  const more = async () => {
    if (busy || next < 0) return;
    busy = true;
    try {
      const r = await fetch(url + '?off=' + next);
      if (r.ok) {
        const j = await r.json();
        pre.insertAdjacentHTML('beforeend', j.html);
        next = j.next;
      } else next = -1;
    } finally { busy = false; }
    check();
  };
  const check = () => {
    if (innerHeight + scrollY > document.documentElement.scrollHeight - 2000) more();
  };
  addEventListener('scroll', check);
  addEventListener('resize', check);
  check();
})();
</script>`, nil
}

func handleHex(w http.ResponseWriter, r *http.Request) {
	e, _, ok := lookup(r, "/hex/")
	if !ok {
		http.NotFound(w, r)
		return
	}
	off, err := strconv.ParseInt(r.URL.Query().Get("off"), 10, 64)
	if err != nil || off < 0 || off%hexWidth != 0 {
		http.Error(w, "bad offset", http.StatusBadRequest)
		return
	}
	f, err := os.Open(e.path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || off >= info.Size() {
		http.Error(w, "offset out of range", http.StatusBadRequest)
		return
	}
	data, next, err := readChunk(f, info.Size(), off)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"html": hexRows(data, off), "next": next})
}
