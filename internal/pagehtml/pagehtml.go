// Package pagehtml turns a page (Markdown with media:ID embeds) into a
// standalone HTML file that can be sent to someone else. Media is either
// embedded in the file (data: URIs) or written next to it in a media/ folder
// (a zip), which keeps big videos seekable.
package pagehtml

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	ghtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"aicw/internal/media"
)

// EmbedLimit is the most media (in bytes) that is embedded into a single
// HTML file. Above it the export becomes a zip with a media/ folder.
const EmbedLimit = 40 << 20

type Media struct {
	ID   int64
	Name string
	Kind string // image | video | audio | pdf | file
	Mime string
	Path string // file on disk
	Size int64
}

// Resolver looks up a media row by id.
type Resolver func(id int64) (Media, bool)

// Rel is the path of a media file inside a zip export.
func Rel(m Media) string {
	return "media/" + strconv.FormatInt(m.ID, 10) + "-" + media.SafeName(m.Name)
}

// Plan renders the page. The result says whether it needs a zip (too much
// media to embed) and which media files it uses.
type Plan struct {
	Used   []Media
	NeedZ  bool
	render func(embed bool) (string, error)
}

// New parses the page once and decides between a single file and a zip.
func New(title, summary, body string, res Resolver) (*Plan, error) {
	p := &Plan{}
	seen := map[int64]bool{}
	var total int64
	use := func(id int64) (Media, bool) {
		m, ok := res(id)
		if !ok {
			return m, false
		}
		if !seen[id] {
			seen[id] = true
			p.Used = append(p.Used, m)
			total += m.Size
		}
		return m, true
	}
	// A dry render collects which media the page uses.
	if _, err := convert(body, func(id int64) (Media, bool) { return use(id) }, false); err != nil {
		return nil, err
	}
	p.NeedZ = total > EmbedLimit
	p.render = func(embed bool) (string, error) {
		content, err := convert(body, res, embed)
		if err != nil {
			return "", err
		}
		return page(title, summary, content), nil
	}
	return p, nil
}

// HTML renders the page; embed puts media into the file as data: URIs.
func (p *Plan) HTML(embed bool) (string, error) { return p.render(embed) }

// WriteZip writes index.html plus the media files.
func (p *Plan) WriteZip(w io.Writer) error {
	doc, err := p.render(false)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(w)
	f, err := zw.Create("index.html")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, doc); err != nil {
		return err
	}
	for _, m := range p.Used {
		src, err := os.Open(m.Path)
		if err != nil {
			continue // a missing file is left out; the page shows it as a broken link
		}
		// Media is already compressed; store it as is so players can read it fast.
		h := &zip.FileHeader{Name: Rel(m), Method: zip.Store}
		out, err := zw.CreateHeader(h)
		if err == nil {
			_, err = io.Copy(out, src)
		}
		src.Close()
		if err != nil {
			return err
		}
	}
	return zw.Close()
}

func convert(body string, res Resolver, embed bool) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(ghtml.WithHardWraps()), // raw HTML in the page stays escaped
		goldmark.WithRenderer(renderer.NewRenderer(renderer.WithNodeRenderers(
			util.Prioritized(ghtml.NewRenderer(), 1000),
			util.Prioritized(&mediaRenderer{res: res, embed: embed}, 100),
		))),
	)
	var b bytes.Buffer
	if err := md.Convert([]byte(body), &b); err != nil {
		return "", err
	}
	return b.String(), nil
}

type mediaRenderer struct {
	res   Resolver
	embed bool
}

func (r *mediaRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.image)
	reg.Register(ast.KindLink, r.link)
}

func mediaID(dest []byte) (int64, bool) {
	s := string(dest)
	if !strings.HasPrefix(s, "media:") {
		return 0, false
	}
	id, err := strconv.ParseInt(s[len("media:"):], 10, 64)
	return id, err == nil
}

func (r *mediaRenderer) src(m Media) string {
	if !r.embed {
		return html.EscapeString(Rel(m))
	}
	data, err := os.ReadFile(m.Path)
	if err != nil {
		return ""
	}
	return "data:" + m.Mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// figure renders one media item as the right element.
func (r *mediaRenderer) figure(w util.BufWriter, m Media, caption string) {
	src := r.src(m)
	cap := html.EscapeString(caption)
	w.WriteString(`<figure class="m ` + html.EscapeString(m.Kind) + `">`)
	switch m.Kind {
	case "image":
		fmt.Fprintf(w, `<img src="%s" alt="%s" loading="lazy">`, src, cap)
	case "video":
		fmt.Fprintf(w, `<video src="%s" controls playsinline preload="metadata"></video>`, src)
	case "audio":
		fmt.Fprintf(w, `<audio src="%s" controls preload="metadata"></audio>`, src)
	default:
		fmt.Fprintf(w, `<a class="file" href="%s" download="%s">%s</a>`, src, html.EscapeString(media.SafeName(m.Name)), html.EscapeString(m.Name))
	}
	if caption != "" && m.Kind != "file" && m.Kind != "pdf" {
		w.WriteString(`<figcaption>` + cap + `</figcaption>`)
	}
	w.WriteString(`</figure>`)
}

func (r *mediaRenderer) image(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	img := n.(*ast.Image)
	id, ok := mediaID(img.Destination)
	caption := string(img.Text(source))
	if !ok {
		w.WriteString(`<span class="gone">[image: ` + html.EscapeString(caption) + `]</span>`)
		return ast.WalkSkipChildren, nil
	}
	if m, found := r.res(id); found {
		r.figure(w, m, caption)
	} else {
		w.WriteString(`<span class="gone">[missing media: ` + html.EscapeString(caption) + `]</span>`)
	}
	return ast.WalkSkipChildren, nil
}

// Links to media become embeds like in the app; other links are shown as text
// (the app does the same, and an exported file should not carry live links).
func (r *mediaRenderer) link(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	l := n.(*ast.Link)
	if id, ok := mediaID(l.Destination); ok {
		if entering {
			if m, found := r.res(id); found {
				r.figure(w, m, string(l.Text(source)))
			}
		}
		return ast.WalkSkipChildren, nil
	}
	if entering {
		w.WriteString(`<span class="lnk" title="` + html.EscapeString(string(l.Destination)) + `">`)
	} else {
		w.WriteString(`</span>`)
	}
	return ast.WalkContinue, nil
}

func page(title, summary, content string) string {
	var b strings.Builder
	t := html.EscapeString(title)
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>` + t + `</title><style>` + css + `</style></head><body><main><h1>` + t + `</h1>`)
	if strings.TrimSpace(summary) != "" {
		b.WriteString(`<p class="lede">` + html.EscapeString(summary) + `</p>`)
	}
	b.WriteString(content)
	b.WriteString(`</main><script>` + js + `</script></body></html>`)
	return b.String()
}

const css = `:root{color-scheme:light dark;--bg:#fff;--fg:#1b1d21;--mute:#666d78;--line:#e3e5e9;--raise:#f5f6f8;--accent:#4f46e5}
@media(prefers-color-scheme:dark){:root{--bg:#101216;--fg:#e8eaee;--mute:#9aa1ad;--line:#2a2e36;--raise:#181b21;--accent:#8b8cf8}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--fg);font:16px/1.7 system-ui,-apple-system,Segoe UI,Roboto,sans-serif}
main{max-width:52rem;margin:0 auto;padding:3rem 1.25rem 6rem}h1{font-size:2.6rem;line-height:1.15;margin:0 0 .5rem}.lede{color:var(--mute);font-size:1.15rem;margin:0 0 2rem}
h2{font-size:1.7rem;margin:2.5rem 0 .5rem}h3{font-size:.85rem;letter-spacing:.08em;text-transform:uppercase;color:var(--accent);margin:1.8rem 0 .3rem}
blockquote{margin:1.2rem 0;padding:.6rem 1.2rem;background:var(--raise);border:1px solid var(--line);border-left:4px solid var(--accent);border-radius:12px}
pre{overflow:auto;background:var(--raise);border:1px solid var(--line);border-radius:10px;padding:1rem;font-size:.85rem}code{background:var(--raise);border-radius:4px;padding:.1em .3em}pre code{background:none;padding:0}
table{border-collapse:collapse;width:100%;display:block;overflow:auto}th,td{border-bottom:1px solid var(--line);padding:.5rem .7rem;text-align:left}th{font-size:.75rem;text-transform:uppercase;color:var(--mute)}
hr{border:0;border-top:1px solid var(--line);margin:2rem 0}.lnk{text-decoration:underline dotted}.gone{color:var(--mute)}
figure.m{position:relative;margin:1.5rem 0;overflow:hidden;border:1px solid var(--line);border-radius:16px;background:var(--raise)}
figure.m img,figure.m video{display:block;width:100%;max-height:70vh;object-fit:contain;margin:0 auto}figure.m video{background:#000}figure.m audio{display:block;width:calc(100% - 2rem);margin:1rem}
figure.m figcaption{position:absolute;left:.7rem;bottom:.7rem;background:rgba(0,0,0,.7);color:#fff;font-size:.7rem;font-weight:600;padding:.2rem .5rem;border-radius:6px}figure.audio figcaption,figure.m .file{position:static;display:block;padding:1rem}
figure.audio figcaption{background:none;color:var(--mute)}
@media print{figure.m video,figure.m audio{display:none}}`

// Only one video/audio plays at a time, and playback stops when scrolled away.
const js = `(function(){var cur=null;document.addEventListener('play',function(e){if(cur&&cur!==e.target)cur.pause();cur=e.target},true);
if('IntersectionObserver'in window){var io=new IntersectionObserver(function(es){es.forEach(function(x){if(!x.isIntersecting&&!x.target.paused)x.target.pause()})},{threshold:.1});
document.querySelectorAll('video').forEach(function(v){io.observe(v)})}})();`
