package pagehtml

import (
	"strings"
	"testing"
)

func TestViewerAssetsSafeToInline(t *testing.T) {
	if viewerJS == "" || viewerCSS == "" {
		t.Fatal("viewer assets not embedded")
	}
	if strings.Contains(strings.ToLower(viewerJS), "</script") || strings.Contains(strings.ToLower(viewerCSS), "</style") {
		t.Fatal("viewer assets would break out of their tag when inlined")
	}
}

func TestZipExportUsesRelativeMediaPath(t *testing.T) {
	res := func(id int64) (Media, bool) { return Media{ID: id, Name: "a b.mp4", Kind: "video", Mime: "video/mp4"}, true }
	p, err := New("T", "", "![c](media:7)", res)
	if err != nil {
		t.Fatal(err)
	}
	h, err := p.HTML(false)
	if err != nil || !strings.Contains(h, `data-src="media/7-a b.mp4"`) {
		t.Fatalf("%v %q", err, h[strings.Index(h, "<figure"):strings.Index(h, "<figure")+160])
	}
}
