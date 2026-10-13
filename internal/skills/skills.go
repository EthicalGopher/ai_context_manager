// Package skills discovers, previews and renders agent skills. Discovery is
// strictly read-only: skill contents are parsed as text and never executed.
package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aicw/internal/contextbuilder"
	"aicw/internal/safety"
)

const (
	SourceProject = "project"
	SourceGlobal  = "user-global"
)

// skillDirs lists where each agent family looks for skills. Conventions differ
// per tool and change over time, so this table is documentation of what AICW
// inspects, not a claim about what every tool loads.
var skillDirs = []struct{ Dir, Agent string }{
	{".claude/skills", "claude"},
	{".agents/skills", "codex/generic"},
	{".codex/skills", "codex"},
	{".gemini/skills", "gemini"},
	{"skills", "generic"},
}

type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Path        string   `json:"path"` // absolute SKILL.md
	Dir         string   `json:"dir"`
	RelPath     string   `json:"relPath,omitempty"`
	Source      string   `json:"source"`
	Agent       string   `json:"agent"`
	Managed     bool     `json:"managed"`         // installed by AICW
	TemplateVer int      `json:"templateVersion"` // for managed skills
	Hash        string   `json:"hash"`
	Warnings    []string `json:"warnings"`
}

type Inventory struct {
	Skills     []Skill  `json:"skills"`
	Duplicates []string `json:"duplicates"` // same name, different content
	Conflicts  []string `json:"conflicts"`  // alias of Duplicates kept for UI clarity
}

const maxSkillBytes = 256 * 1024

// Discover reads project skills under root and, if home != "", user-global
// skills. It follows no symlink that leaves its base directory.
func Discover(root, home string) (*Inventory, error) {
	inv := &Inventory{Skills: []Skill{}, Duplicates: []string{}, Conflicts: []string{}}
	scan := func(base, source string) {
		for _, d := range skillDirs {
			dir := filepath.Join(base, d.Dir)
			ents, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range ents {
				skillMD := filepath.Join(dir, e.Name(), "SKILL.md")
				if e.Type()&fs.ModeSymlink != 0 {
					if real, err := filepath.EvalSymlinks(filepath.Join(dir, e.Name())); err != nil || !safety.Contains(base, real) {
						inv.Skills = append(inv.Skills, Skill{Name: e.Name(), Path: skillMD, Source: source, Agent: d.Agent, Warnings: []string{"symlink leaves its base directory; not read"}})
						continue
					}
				}
				fi, err := os.Stat(skillMD)
				if err != nil || fi.IsDir() {
					continue
				}
				s := Skill{Name: e.Name(), Path: skillMD, Dir: filepath.Join(dir, e.Name()), Source: source, Agent: d.Agent, Warnings: []string{}}
				if rel, err := filepath.Rel(base, skillMD); err == nil && source == SourceProject {
					s.RelPath = filepath.ToSlash(rel)
				}
				if fi.Size() > maxSkillBytes {
					s.Warnings = append(s.Warnings, "SKILL.md is larger than 256 KB; not parsed")
				} else if b, err := os.ReadFile(skillMD); err == nil {
					text := string(b)
					if n, desc := contextbuilder.ParseFrontmatter(text); n != "" {
						s.Name, s.Description = n, desc
					} else {
						s.Description = desc
					}
					s.Hash = hashOf(text)
					s.Managed, s.TemplateVer = managedInfo(text)
					if safety.LooksInjected(text) {
						s.Warnings = append(s.Warnings, "contains text resembling prompt-injection or a pipe-to-shell command")
					}
					if safety.LooksSecret(text) {
						s.Warnings = append(s.Warnings, "may contain credentials")
					}
				}
				if hasScripts(s.Dir) {
					s.Warnings = append(s.Warnings, "bundles scripts; AICW never runs them, review before letting an agent do so")
				}
				inv.Skills = append(inv.Skills, s)
			}
		}
	}
	scan(root, SourceProject)
	if home != "" {
		scan(home, SourceGlobal)
	}
	sort.SliceStable(inv.Skills, func(i, j int) bool {
		if inv.Skills[i].Source != inv.Skills[j].Source {
			return inv.Skills[i].Source < inv.Skills[j].Source
		}
		return inv.Skills[i].Path < inv.Skills[j].Path
	})
	byName := map[string]Skill{}
	seen := map[string]bool{}
	for _, s := range inv.Skills {
		if s.Hash == "" {
			continue
		}
		if prev, ok := byName[s.Name]; ok && prev.Hash != s.Hash {
			msg := fmt.Sprintf("%q differs between %s and %s", s.Name, prev.Path, s.Path)
			if !seen[msg] {
				inv.Duplicates = append(inv.Duplicates, msg)
				seen[msg] = true
			}
		} else if !ok {
			byName[s.Name] = s
		}
	}
	inv.Conflicts = inv.Duplicates
	return inv, nil
}

func hashOf(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:6]) }

func hasScripts(dir string) bool {
	found := false
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return fs.SkipDir
		}
		if !d.IsDir() {
			switch strings.ToLower(filepath.Ext(p)) {
			case ".sh", ".py", ".js", ".mjs", ".ts", ".rb", ".ps1", ".bat":
				found = true
			}
		}
		return nil
	})
	return found
}

// ---- import preview ----

type ImportFile struct {
	Rel      string   `json:"rel"`
	Size     int64    `json:"size"`
	Warnings []string `json:"warnings"`
}

type ImportPreview struct {
	Source   string       `json:"source"`
	Name     string       `json:"name"`
	Files    []ImportFile `json:"files"`
	Warnings []string     `json:"warnings"`
}

const (
	maxImportFiles = 50
	maxImportBytes = 2 * 1024 * 1024
)

// PreviewImport lists what importing a local skill directory would copy.
// It reads but never executes or modifies anything.
func PreviewImport(src string) (*ImportPreview, error) {
	real, err := filepath.EvalSymlinks(src)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(real)
	if err != nil || !st.IsDir() {
		return nil, errors.New("not a directory")
	}
	if _, err := os.Stat(filepath.Join(real, "SKILL.md")); err != nil {
		return nil, errors.New("directory has no SKILL.md")
	}
	p := &ImportPreview{Source: real, Name: filepath.Base(real), Files: []ImportFile{}, Warnings: []string{"Skills are instructions an AI agent may follow. Read them before installing; they can contain untrusted commands or prompt-injection."}}
	var total int64
	err = filepath.WalkDir(real, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(real, path)
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			p.Warnings = append(p.Warnings, "symlink skipped: "+rel)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		f := ImportFile{Rel: filepath.ToSlash(rel), Size: info.Size(), Warnings: []string{}}
		total += info.Size()
		if len(p.Files) >= maxImportFiles || total > maxImportBytes {
			return errors.New("skill is too large to import (limit 50 files / 2 MB)")
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".sh", ".py", ".js", ".mjs", ".ts", ".rb", ".ps1", ".bat", "":
			f.Warnings = append(f.Warnings, "executable-type file")
		}
		if info.Size() < maxSkillBytes {
			if b, err := os.ReadFile(path); err == nil {
				if safety.LooksInjected(string(b)) {
					f.Warnings = append(f.Warnings, "resembles prompt-injection")
				}
				if safety.LooksSecret(string(b)) {
					f.Warnings = append(f.Warnings, "may contain credentials")
				}
			}
		}
		p.Files = append(p.Files, f)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(p.Files, func(i, j int) bool { return p.Files[i].Rel < p.Files[j].Rel })
	return p, nil
}

// ReadImportFile returns a previewed file's text, only if it was listed.
func ReadImportFile(src, rel string) (string, error) {
	full, err := safety.SafeJoin(src, rel)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(full)
	return string(b), err
}
