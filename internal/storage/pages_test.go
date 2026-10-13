package storage

import "testing"

func TestPagesUpsertAppendOrderVersions(t *testing.T) {
	s, w := open(t)
	a, err := s.PutPage(w.ID, "Character", "lede", "# Meet\n", "user")
	if err != nil || a.Sort != 1 {
		t.Fatalf("%+v %v", a, err)
	}
	b, _ := s.PutPage(w.ID, "Locations", "", "x", "agent")
	if b.Sort != 2 {
		t.Fatalf("%+v", b)
	}
	// Same title in a different case updates the same page and keeps a version.
	a2, _ := s.PutPage(w.ID, "character", "lede2", "# Meet v2\n", "agent")
	if a2.ID != a.ID || a2.Title != "character" || a2.UpdatedBy != "agent" {
		t.Fatalf("%+v", a2)
	}
	v, _ := s.PageVersions(a.ID)
	if len(v) != 1 || v[0].Body != "# Meet\n" || v[0].SavedBy != "user" {
		t.Fatalf("%+v", v)
	}
	p, _ := s.AppendToPage(w.ID, "CHARACTER", "![shot](media:3)", "agent")
	if p.ID != a.ID || p.Body != "# Meet v2\n\n![shot](media:3)" {
		t.Fatalf("%q", p.Body)
	}
	n, _ := s.AppendToPage(w.ID, "Brand new", "hello", "user")
	if n.Body != "hello" || n.Sort != 3 {
		t.Fatalf("%+v", n)
	}
	s.ReorderPages(w.ID, []int64{n.ID, b.ID, a.ID})
	l, _ := s.Pages(w.ID)
	if l[0].ID != n.ID || l[2].ID != a.ID {
		t.Fatalf("%+v", l)
	}
	other, _, _ := s.UpsertWorkspace("/q", "q")
	if _, err := s.PutPage(other.ID, "Character", "", "mine", "user"); err != nil {
		t.Fatal("same title in another workspace must be allowed:", err)
	}
	if _, err := s.PutPage(w.ID, "  ", "", "", "user"); err == nil {
		t.Fatal("empty title accepted")
	}
	s.DeletePage(a.ID)
	if v, _ := s.PageVersions(a.ID); len(v) != 0 {
		t.Fatal("versions must cascade")
	}
}
