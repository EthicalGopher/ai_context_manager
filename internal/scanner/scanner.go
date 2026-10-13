// Package scanner performs read-only discovery of AI guidance, skills and
// documentation inside a workspace.
package scanner

import (
	"bufio"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aicw/internal/safety"
)

// Categories used in the inventory.
const (
	CatInstruction  = "instruction"
	CatSkill        = "skill"
	CatArchitecture = "architecture"
	CatTask         = "task-notes"
	CatSession      = "session-summary"
	CatGenerated    = "generated"
	CatDoc          = "documentation"
)

// Options controls a scan. Zero values are replaced by defaults.
type Options struct {
	IgnoreDirs   []string `json:"ignoreDirs"`
	IgnoreGlobs  []string `json:"ignoreGlobs"`
	MaxFileBytes int64    `json:"maxFileBytes"`
	MaxFiles     int      `json:"maxFiles"`
	MaxDepth     int      `json:"maxDepth"`
	Rules        []Rule   `json:"rules"`
}

// Rule maps a path pattern (relative, slash separated, path.Match syntax; a
// trailing "/" matches a directory prefix) to a category. Rules are
// configurable because tool conventions change.
type Rule struct {
	Pattern  string `json:"pattern"`
	Category string `json:"category"`
}

// Item is one discovered file.
type Item struct {
	Path      string    `json:"path"` // relative, slash separated
	Category  string    `json:"category"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"modTime"`
	Secret    bool      `json:"secret"`
	Injection bool      `json:"injection"`
	TooLarge  bool      `json:"tooLarge"`
	Action    string    `json:"action"` // proposed action
	Warning   string    `json:"warning,omitempty"`
}

// GitInfo summarises repository state.
type GitInfo struct {
	IsRepo  bool   `json:"isRepo"`
	Branch  string `json:"branch"`
	Dirty   int    `json:"dirty"`
	Message string `json:"message,omitempty"`
}

// Result is the scan output.
type Result struct {
	Root       string   `json:"root"`
	Items      []Item   `json:"items"`
	Git        GitInfo  `json:"git"`
	State      string   `json:"state"` // empty | initialized | ai-configured
	Warnings   []string `json:"warnings"`
	Truncated  bool     `json:"truncated"`
	Duplicates []string `json:"duplicates"`
	Tree       []string `json:"tree"` // first two levels
}

func DefaultOptions() Options {
	return Options{
		IgnoreDirs:   []string{"node_modules", ".git", "vendor", "dist", "build", "target", "__pycache__", ".venv", "venv", ".next", ".cache", ".idea", ".gradle", "coverage", ".aicw/history"},
		MaxFileBytes: 512 * 1024,
		MaxFiles:     5000,
		MaxDepth:     8,
		Rules:        DefaultRules(),
	}
}

// DefaultRules is the documented discovery list (see docs/ARCHITECTURE.md).
func DefaultRules() []Rule {
	return []Rule{
		{"AGENTS.md", CatInstruction}, {"CLAUDE.md", CatInstruction}, {"GEMINI.md", CatInstruction},
		{"CODEX.md", CatInstruction}, {".cursorrules", CatInstruction}, {".cursor/rules/", CatInstruction},
		{".github/copilot-instructions.md", CatInstruction}, {".github/instructions/", CatInstruction},
		{"README.md", CatDoc}, {"CONTRIBUTING.md", CatDoc},
		{"ARCHITECTURE.md", CatArchitecture}, {"docs/ARCHITECTURE.md", CatArchitecture},
		{".claude/skills/", CatSkill}, {".agents/skills/", CatSkill}, {".codex/skills/", CatSkill},
		{".gemini/skills/", CatSkill}, {"skills/", CatSkill},
		{".claude/", CatInstruction}, {".codex/", CatInstruction}, {".gemini/", CatInstruction}, {".agents/", CatInstruction},
		{".aicw/sessions/", CatSession}, {".aicw/context/current-task.md", CatTask}, {".aicw/", CatGenerated},
		{"context/", CatTask}, {"docs/", CatDoc}, {".github/", CatDoc},
	}
}

func merge(o Options) Options {
	d := DefaultOptions()
	if o.IgnoreDirs == nil {
		o.IgnoreDirs = d.IgnoreDirs
	}
	if o.MaxFileBytes == 0 {
		o.MaxFileBytes = d.MaxFileBytes
	}
	if o.MaxFiles == 0 {
		o.MaxFiles = d.MaxFiles
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = d.MaxDepth
	}
	if o.Rules == nil {
		o.Rules = d.Rules
	}
	return o
}

// Classify returns the category for a relative path, or "" if no rule applies.
func Classify(rel string, rules []Rule) string {
	for _, r := range rules {
		if strings.HasSuffix(r.Pattern, "/") {
			if strings.HasPrefix(rel, r.Pattern) {
				return r.Category
			}
			continue
		}
		if ok, _ := path.Match(r.Pattern, rel); ok {
			return r.Category
		}
	}
	return ""
}

// Scan inspects root without modifying anything.
func Scan(root string, opts Options) (*Result, error) {
	opts = merge(opts)
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	res := &Result{Root: realRoot, Items: []Item{}, Warnings: []string{}, Duplicates: []string{}, Tree: []string{}}
	ig := loadGitignore(realRoot)
	count := 0

	walkErr := filepath.WalkDir(realRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				res.Warnings = append(res.Warnings, "cannot read directory: "+relOf(realRoot, p))
				return fs.SkipDir
			}
			return nil
		}
		if p == realRoot {
			return nil
		}
		rel := relOf(realRoot, p)
		depth := strings.Count(rel, "/") + 1
		if d.IsDir() {
			if depth <= 2 {
				res.Tree = append(res.Tree, rel+"/")
			}
			if depth > opts.MaxDepth || ignoredDir(rel, d.Name(), opts) || (ig.match(rel, true) && !isGuidanceDir(rel, opts)) {
				return fs.SkipDir
			}
			return nil
		}
		if depth <= 2 {
			res.Tree = append(res.Tree, rel)
		}
		if d.Type()&fs.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(p)
			if err != nil || !safety.Contains(realRoot, target) {
				res.Warnings = append(res.Warnings, "skipped symlink pointing outside workspace: "+rel)
				return nil
			}
			if st, err := os.Stat(target); err != nil || st.IsDir() {
				return nil
			}
		}
		cat := Classify(rel, opts.Rules)
		isMD := strings.HasSuffix(strings.ToLower(rel), ".md")
		if cat == "" && !isMD {
			return nil
		}
		if cat == "" {
			cat = CatDoc
		}
		if matchGlobs(rel, opts.IgnoreGlobs) || (ig.match(rel, false) && !isGuidanceDir(rel, opts)) {
			return nil
		}
		count++
		if count > opts.MaxFiles {
			res.Truncated = true
			return fs.SkipAll
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		it := Item{Path: rel, Category: refine(rel, cat), Size: info.Size(), ModTime: info.ModTime(), Action: "keep"}
		it.Secret = safety.IsSecretFile(rel)
		if it.Secret {
			it.Warning = "likely secret file; excluded from context"
			it.Action = "exclude"
		} else if info.Size() > opts.MaxFileBytes {
			it.TooLarge = true
			it.Action = "exclude"
			it.Warning = "larger than file-size limit"
		} else if b := readHead(p, 64*1024); b != "" {
			if safety.LooksSecret(b) {
				it.Secret, it.Action, it.Warning = true, "exclude", "content looks like it contains credentials; excluded from context"
			} else if safety.LooksInjected(b) {
				it.Injection, it.Warning = true, "contains text resembling prompt-injection; review before use"
			}
		}
		res.Items = append(res.Items, it)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Slice(res.Items, func(i, j int) bool { return res.Items[i].Path < res.Items[j].Path })
	sort.Strings(res.Tree)
	res.Git = gitInfo(realRoot)
	res.State = classifyState(realRoot, res.Items)
	res.Duplicates = findDuplicates(res.Items)
	return res, nil
}

// refine sharpens the generic documentation category by file name.
func refine(rel, cat string) string {
	if cat != CatDoc {
		return cat
	}
	l := strings.ToLower(rel)
	switch {
	case strings.Contains(l, "architecture"):
		return CatArchitecture
	case strings.Contains(l, "handoff") || strings.Contains(l, "session"):
		return CatSession
	case strings.Contains(l, "task") || strings.Contains(l, "todo"):
		return CatTask
	}
	return cat
}

// isGuidanceDir keeps hidden guidance dirs (.claude, .github, ...) visible
// even when .gitignore would otherwise hide them.
func isGuidanceDir(rel string, o Options) bool {
	top := strings.SplitN(rel, "/", 2)[0]
	for _, r := range o.Rules {
		if strings.HasPrefix(r.Pattern, ".") && strings.HasPrefix(r.Pattern, top+"/") {
			return true
		}
	}
	return false
}

func ignoredDir(rel, name string, o Options) bool {
	for _, d := range o.IgnoreDirs {
		if d == name || d == rel {
			return true
		}
	}
	return false
}

func matchGlobs(rel string, globs []string) bool {
	for _, g := range globs {
		if ok, _ := path.Match(g, rel); ok {
			return true
		}
		if ok, _ := path.Match(g, path.Base(rel)); ok {
			return true
		}
	}
	return false
}

func relOf(root, p string) string {
	r, _ := filepath.Rel(root, p)
	return filepath.ToSlash(r)
}

func readHead(p string, n int) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, n)
	m, _ := f.Read(buf)
	return string(buf[:m])
}

func classifyState(root string, items []Item) string {
	for _, it := range items {
		if it.Category == CatInstruction || it.Category == CatSkill {
			return "ai-configured"
		}
	}
	ents, err := os.ReadDir(root)
	if err != nil {
		return "empty"
	}
	for _, e := range ents {
		if e.Name() != ".git" {
			return "initialized"
		}
	}
	return "empty"
}

// findDuplicates reports instruction files with the same well-known name in
// different directories, which commonly drift apart.
func findDuplicates(items []Item) []string {
	seen := map[string]string{}
	var dups []string
	for _, it := range items {
		key := strings.ToLower(path.Base(it.Path))
		if key != "agents.md" && key != "claude.md" && key != "gemini.md" {
			continue
		}
		if prev, ok := seen[key]; ok {
			dups = append(dups, prev+" ~ "+it.Path)
		}
		seen[key] = it.Path
	}
	return dups
}

func gitInfo(root string) GitInfo {
	if _, err := exec.LookPath("git"); err != nil {
		return GitInfo{Message: "git not installed"}
	}
	run := func(args ...string) (string, error) {
		c := exec.Command("git", append([]string{"-C", root}, args...)...)
		c.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
		out, err := c.Output()
		return strings.TrimSpace(string(out)), err
	}
	top, err := run("rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return GitInfo{Message: "not a git repository"}
	}
	gi := GitInfo{IsRepo: true}
	if b, err := run("symbolic-ref", "--short", "HEAD"); err == nil && b != "" {
		gi.Branch = b // works before the first commit
	} else if b, err := run("rev-parse", "--short", "HEAD"); err == nil {
		gi.Branch = "detached@" + b
	}
	if s, err := run("status", "--porcelain"); err == nil && s != "" {
		gi.Dirty = len(strings.Split(s, "\n"))
	}
	return gi
}

// ---- minimal .gitignore support (root-level file only) ----

type gitignore struct{ pats []gipat }
type gipat struct {
	pat     string
	dirOnly bool
	anchor  bool
	neg     bool
}

func loadGitignore(root string) *gitignore {
	g := &gitignore{}
	f, err := os.Open(filepath.Join(root, ".gitignore"))
	if err != nil {
		return g
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		p := gipat{}
		if strings.HasPrefix(l, "!") {
			p.neg, l = true, l[1:]
		}
		if strings.HasSuffix(l, "/") {
			p.dirOnly, l = true, strings.TrimSuffix(l, "/")
		}
		if strings.HasPrefix(l, "/") {
			p.anchor, l = true, l[1:]
		} else if strings.Contains(l, "/") {
			p.anchor = true
		}
		p.pat = l
		g.pats = append(g.pats, p)
	}
	return g
}

func (g *gitignore) match(rel string, isDir bool) bool {
	ignored := false
	for _, p := range g.pats {
		if p.dirOnly && !isDir {
			continue // files inside an ignored dir are skipped with the dir
		}
		var ok bool
		if p.anchor {
			ok, _ = path.Match(p.pat, rel)
		} else {
			ok, _ = path.Match(p.pat, path.Base(rel))
		}
		if ok {
			ignored = !p.neg
		}
	}
	return ignored
}
