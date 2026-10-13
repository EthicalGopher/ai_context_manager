// Package safety holds path-resolution, containment and secret-detection
// helpers. Everything that touches user-supplied paths goes through here.
package safety

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ErrNotExist = errors.New("path does not exist")
	ErrEscape   = errors.New("path escapes the workspace")
)

// IsFileError is returned when a file was supplied where a directory is needed.
type IsFileError struct{ Path, Parent string }

func (e *IsFileError) Error() string {
	return fmt.Sprintf("%s is a file, not a directory (parent directory: %s)", e.Path, e.Parent)
}

// ExpandHome expands a leading "~" or "~/".
func ExpandHome(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~"+string(filepath.Separator)) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		return filepath.Join(home, p[2:]), nil
	}
	return p, nil
}

// ResolveDir turns a CLI/UI path into a canonical absolute directory path.
// It never creates or modifies anything.
func ResolveDir(arg, cwd string) (string, error) {
	if strings.TrimSpace(arg) == "" {
		return "", errors.New("empty path")
	}
	p, err := ExpandHome(arg)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(p) {
		if cwd == "" {
			if cwd, err = os.Getwd(); err != nil {
				return "", err
			}
		}
		p = filepath.Join(cwd, p)
	}
	p = filepath.Clean(p)
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrNotExist, p)
		}
		return "", err
	}
	st, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", &IsFileError{Path: real, Parent: filepath.Dir(real)}
	}
	return real, nil
}

// Contains reports whether target is root or lies inside it (lexically).
func Contains(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

// SafeJoin joins a relative path to root and guarantees the result stays
// inside root, including after resolving symlinks of the deepest existing
// ancestor.
func SafeJoin(root, rel string) (string, error) {
	if filepath.IsAbs(rel) {
		return "", ErrEscape
	}
	full := filepath.Join(root, rel)
	if !Contains(root, full) {
		return "", ErrEscape
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	// Resolve the deepest existing ancestor.
	cur := full
	for {
		if _, err := os.Lstat(cur); err == nil {
			break
		}
		next := filepath.Dir(cur)
		if next == cur {
			break
		}
		cur = next
	}
	realCur, err := filepath.EvalSymlinks(cur)
	if err != nil {
		return "", err
	}
	if !Contains(realRoot, realCur) {
		return "", ErrEscape
	}
	return full, nil
}

var secretNames = []string{
	".env", ".env.*", "*.pem", "*.key", "*.p12", "*.pfx", "*.jks", "*.keystore",
	"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519", "*.kdbx", ".npmrc", ".pypirc",
	".netrc", "credentials", "credentials.json", "secrets.*", "*.secret", "*.secrets",
	"service-account*.json", ".htpasswd", "terraform.tfstate", "*.tfvars",
}

var allowedEnvTemplates = map[string]bool{".env.example": true, ".env.sample": true, ".env.template": true}

// IsSecretFile reports whether a file name looks like it holds secrets.
// This is a heuristic and is not perfect.
func IsSecretFile(name string) bool {
	base := strings.ToLower(filepath.Base(name))
	if allowedEnvTemplates[base] {
		return false
	}
	for _, pat := range secretNames {
		if ok, _ := filepath.Match(pat, base); ok {
			return true
		}
	}
	return false
}

var secretContent = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`ghp_[A-Za-z0-9]{30,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{30,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{24,}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{35}`),
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|passwd|password)\s*[:=]\s*["']?[A-Za-z0-9/+_\-]{16,}`),
}

// LooksSecret reports whether text appears to contain credentials. Heuristic only.
func LooksSecret(text string) bool {
	for _, re := range secretContent {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

var injectionPhrases = regexp.MustCompile(`(?i)(ignore (all |any )?(previous|prior|above) (instructions|prompts)|disregard (the )?(system|previous) (prompt|instructions)|you are now (in )?developer mode|curl [^|\n]*\|\s*(ba)?sh)`)

// LooksInjected flags text containing common prompt-injection phrasing.
// Heuristic only; never treat absence of a match as proof of safety.
func LooksInjected(text string) bool { return injectionPhrases.MatchString(text) }
