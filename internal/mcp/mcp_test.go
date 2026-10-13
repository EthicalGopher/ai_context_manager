package mcp

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"aicw/internal/storage"
	"aicw/internal/tools"
)

func run(t *testing.T, lines ...string) []map[string]any {
	s, _ := storage.Open(filepath.Join(t.TempDir(), "a.db"))
	defer s.Close()
	w, _, _ := s.UpsertWorkspace("/p", "p")
	var out bytes.Buffer
	if err := Serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out, tools.Env{Store: s, DataDir: t.TempDir(), WorkspaceID: w.ID}, "test"); err != nil {
		t.Fatal(err)
	}
	var res []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(l), &m) != nil {
			t.Fatalf("bad json line %q", l)
		}
		res = append(res, m)
	}
	return res
}

func TestHandshakeListAndCall(t *testing.T) {
	res := run(t,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"write_note","arguments":{"name":"m","body":"hello"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"read_note","arguments":{"name":"nope"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"bogus"}`,
		`not json`,
	)
	if len(res) != 6 { // notification gets no reply
		t.Fatalf("got %d replies: %v", len(res), res)
	}
	init := res[0]["result"].(map[string]any)
	if init["protocolVersion"] != "2025-03-26" || init["serverInfo"].(map[string]any)["name"] != "aicw" {
		t.Fatalf("%v", init)
	}
	list := res[1]["result"].(map[string]any)["tools"].([]any)
	if len(list) != len(tools.All()) || list[0].(map[string]any)["inputSchema"] == nil {
		t.Fatal("tools/list wrong")
	}
	ok := res[2]["result"].(map[string]any)
	if ok["isError"] != nil || !strings.Contains(ok["content"].([]any)[0].(map[string]any)["text"].(string), `"ok": true`) {
		t.Fatalf("%v", ok)
	}
	if res[3]["result"].(map[string]any)["isError"] != true {
		t.Fatal("tool errors must be reported as isError results")
	}
	if res[4]["error"].(map[string]any)["code"].(float64) != -32601 {
		t.Fatal("unknown method code")
	}
	if res[5]["error"].(map[string]any)["code"].(float64) != -32700 {
		t.Fatal("parse error code")
	}
}
