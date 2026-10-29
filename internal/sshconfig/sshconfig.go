// Package sshconfig reads OpenSSH client config files (~/.ssh/config) far
// enough to list the hosts they define and how to reach them.
package sshconfig

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Entry is one concrete Host alias with its settings resolved the way ssh
// would: the first value obtained for each option wins, across every block
// (wildcards included) that matches the alias.
type Entry struct {
	Alias         string   `json:"alias"`
	HostName      string   `json:"hostName"`      // falls back to Alias
	Port          int      `json:"port"`          // 22 when unset
	User          string   `json:"user"`          // empty when unset
	IdentityFiles []string `json:"identityFiles"` // expanded paths, in config order
}

// maxDepth matches OpenSSH's limit on nested Include directives.
const maxDepth = 16

type option struct {
	key  string // lower-cased keyword
	args []string
}

type block struct {
	patterns []string // nil applies to every host (options before the first Host)
	match    bool     // Match blocks can't be evaluated here, so they never apply
	options  []option
}

type parser struct {
	dir    string // relative Include paths resolve against this (~/.ssh)
	home   string
	blocks []*block
}

// Parse reads the config at path and every file it Includes. Relative Include
// paths resolve against the directory holding path, as they do for ~/.ssh/config.
func Parse(path string) ([]Entry, error) {
	home, _ := os.UserHomeDir()
	p := &parser{dir: filepath.Dir(path), home: home}
	cur := &block{}
	p.blocks = append(p.blocks, cur)
	if _, err := p.parseFile(path, cur, 0); err != nil {
		return nil, err
	}

	var out []Entry
	seen := map[string]bool{}
	for _, b := range p.blocks {
		for _, pat := range b.patterns {
			if isPattern(pat) || seen[strings.ToLower(pat)] {
				continue
			}
			seen[strings.ToLower(pat)] = true
			out = append(out, p.resolve(pat))
		}
	}
	return out, nil
}

// parseFile appends the blocks of one file. cur is the block in effect where
// the file was included; options before the file's first Host land in it.
// It returns the block in effect at the end of the file.
func (p *parser) parseFile(path string, cur *block, depth int) (*block, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		key, args, err := splitLine(sc.Text())
		if err != nil {
			return nil, fmt.Errorf("%s baris %d: %w", path, n, err)
		}
		switch key {
		case "":
			continue
		case "host":
			cur = &block{patterns: args}
			p.blocks = append(p.blocks, cur)
		case "match":
			cur = &block{match: true}
			p.blocks = append(p.blocks, cur)
		case "include":
			if depth+1 >= maxDepth {
				return nil, fmt.Errorf("%s baris %d: Include terlalu dalam", path, n)
			}
			for _, arg := range args {
				if cur, err = p.include(arg, cur, depth+1); err != nil {
					return nil, err
				}
			}
		default:
			cur.options = append(cur.options, option{key, args})
		}
	}
	return cur, sc.Err()
}

// include parses every file matching pattern. Like ssh, the including block
// stays in effect for the lines after the Include.
func (p *parser) include(pattern string, cur *block, depth int) (*block, error) {
	pattern = p.expandHome(pattern)
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(p.dir, pattern)
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("pola Include tidak valid %q: %w", pattern, err)
	}
	outer := cur
	for _, m := range matches {
		if info, err := os.Stat(m); err != nil || info.IsDir() {
			continue
		}
		if cur, err = p.parseFile(m, cur, depth); err != nil {
			return nil, err
		}
	}
	if cur != outer {
		cur = &block{patterns: outer.patterns, match: outer.match}
		p.blocks = append(p.blocks, cur)
	}
	return cur, nil
}

func (p *parser) resolve(alias string) Entry {
	e := Entry{Alias: alias}
	var hostName, port string
	var identities []string
	for _, b := range p.blocks {
		if !b.applies(alias) {
			continue
		}
		for _, o := range b.options {
			if len(o.args) == 0 {
				continue
			}
			switch o.key {
			case "hostname":
				if hostName == "" {
					hostName = o.args[0]
				}
			case "port":
				if port == "" {
					port = o.args[0]
				}
			case "user":
				if e.User == "" {
					e.User = o.args[0]
				}
			case "identityfile":
				identities = append(identities, o.args[0])
			}
		}
	}

	e.HostName = alias
	if hostName != "" {
		e.HostName = expandTokens(hostName, map[byte]string{'h': alias})
	}
	e.Port = 22
	if n, err := strconv.Atoi(port); err == nil && n > 0 && n <= 65535 {
		e.Port = n
	}
	for _, id := range identities {
		if strings.EqualFold(id, "none") {
			continue
		}
		id = p.expandHome(expandTokens(id, map[byte]string{'h': e.HostName, 'r': e.User, 'd': p.home, 'n': alias}))
		if !filepath.IsAbs(id) && p.home != "" {
			id = filepath.Join(p.home, id)
		}
		e.IdentityFiles = append(e.IdentityFiles, filepath.Clean(id))
	}
	return e
}

func (b *block) applies(host string) bool {
	if b.match {
		return false
	}
	if b.patterns == nil {
		return true
	}
	host = strings.ToLower(host)
	ok := false
	for _, pat := range b.patterns {
		if neg := strings.HasPrefix(pat, "!"); neg {
			if wildcard(strings.ToLower(pat[1:]), host) {
				return false
			}
		} else if wildcard(strings.ToLower(pat), host) {
			ok = true
		}
	}
	return ok
}

func (p *parser) expandHome(s string) string {
	if p.home != "" && (s == "~" || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`)) {
		return filepath.Join(p.home, s[1:])
	}
	return s
}

// isPattern reports whether a Host argument is a wildcard or negation rather
// than a host someone can connect to.
func isPattern(s string) bool {
	return strings.ContainsAny(s, "*?!")
}

// wildcard matches s against an ssh pattern where * is any run and ? one character.
func wildcard(pat, s string) bool {
	for len(pat) > 0 {
		switch pat[0] {
		case '*':
			for i := len(s); i >= 0; i-- {
				if wildcard(pat[1:], s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if s == "" {
				return false
			}
		default:
			if s == "" || s[0] != pat[0] {
				return false
			}
		}
		pat, s = pat[1:], s[1:]
	}
	return s == ""
}

// expandTokens replaces %x tokens found in vars and %% with %; unknown tokens are kept.
func expandTokens(s string, vars map[byte]string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '%' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		if s[i] == '%' {
			b.WriteByte('%')
		} else if v, ok := vars[s[i]]; ok {
			b.WriteString(v)
		} else {
			b.WriteByte('%')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// splitLine returns the lower-cased keyword and its arguments. Blank lines and
// comments give an empty keyword. The keyword may be separated by "=".
func splitLine(line string) (string, []string, error) {
	line = strings.TrimSpace(line)
	if line == "" || line[0] == '#' {
		return "", nil, nil
	}
	end := strings.IndexAny(line, " \t=")
	if end < 0 {
		return strings.ToLower(line), nil, nil
	}
	key := strings.ToLower(line[:end])
	rest := strings.TrimLeft(line[end:], " \t")
	if strings.HasPrefix(rest, "=") {
		rest = strings.TrimLeft(rest[1:], " \t")
	}
	args, err := splitArgs(rest)
	return key, args, err
}

// splitArgs splits on whitespace, honouring double quotes. An unquoted # at
// the start of an argument ends the line.
func splitArgs(s string) ([]string, error) {
	var args []string
	var cur strings.Builder
	inArg, quoted := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			quoted = !quoted
			inArg = true
		case !quoted && (c == ' ' || c == '\t'):
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		case !quoted && c == '#' && !inArg:
			return args, nil
		default:
			cur.WriteByte(c)
			inArg = true
		}
	}
	if quoted {
		return nil, errors.New("tanda kutip tidak ditutup")
	}
	if inArg {
		args = append(args, cur.String())
	}
	return args, nil
}
