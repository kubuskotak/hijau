package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// resolveProject returns the project id from --project or the config default.
func resolveProject(flagVal string, cfg Config) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if cfg.DefaultProject != "" {
		return cfg.DefaultProject, nil
	}
	return "", fmt.Errorf("no project: pass --project <id> or set a default with `hijau login --project`")
}

// hasText reports whether a translation counts as "done" for completion stats.
func isDone(t Translation, ok bool) bool {
	return ok && strings.TrimSpace(t.Text) != "" && t.State != "untranslated"
}

func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	api := fs.String("api", "http://localhost:8080", "API base URL")
	email := fs.String("email", "", "account email")
	password := fs.String("password", "", "account password")
	token := fs.String("token", "", "use an existing PAT instead of email/password")
	project := fs.String("project", "", "set a default project id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := validateURL(*api); err != nil {
		return err
	}

	cfg := Config{APIURL: *api, DefaultProject: *project}
	switch {
	case *token != "":
		cfg.Token = *token
	case *email != "" && *password != "":
		tok, err := login(*api, *email, *password)
		if err != nil {
			return err
		}
		cfg.Token = tok
	default:
		return fmt.Errorf("provide --token, or --email and --password")
	}

	// Carry over an existing default project if none was passed.
	if cfg.DefaultProject == "" {
		if old, err := loadConfig(); err == nil {
			cfg.DefaultProject = old.DefaultProject
		}
	}
	if err := saveConfig(cfg); err != nil {
		return err
	}
	path, _ := configPath()
	fmt.Printf("Signed in. Token saved to %s\n", path)
	return nil
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	project := fs.String("project", "", "project id (defaults to configured)")
	failUnder := fs.Float64("fail-under", 0, "exit non-zero if any language is below this percent done")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	pid, err := resolveProject(*project, cfg)
	if err != nil {
		return err
	}
	c := newClient(cfg)

	proj, err := c.getProject(pid)
	if err != nil {
		return err
	}
	langs, err := c.listLanguages(pid)
	if err != nil {
		return err
	}
	feed, err := c.editorFeed(pid, 500)
	if err != nil {
		return err
	}
	total := len(feed.Keys)

	fmt.Printf("%s — %d keys\n", proj.Name, total)
	worst := 100.0
	sort.Slice(langs, func(i, j int) bool { return langs[i].Tag < langs[j].Tag })
	for _, l := range langs {
		if l.ID == proj.BaseLanguageID {
			fmt.Printf("  %-8s source\n", l.Tag)
			continue
		}
		done, reviewed := 0, 0
		for _, row := range feed.Keys {
			t, ok := row.Translations[l.ID]
			if isDone(t, ok) {
				done++
			}
			if ok && t.State == "reviewed" {
				reviewed++
			}
		}
		pct := 100.0
		if total > 0 {
			pct = float64(done) / float64(total) * 100
		}
		if pct < worst {
			worst = pct
		}
		fmt.Printf("  %-8s %3.0f%%  %d/%d done, %d reviewed\n", l.Tag, pct, done, total, reviewed)
	}
	if *failUnder > 0 && worst < *failUnder {
		return fmt.Errorf("coverage %.0f%% is below --fail-under %.0f%%", worst, *failUnder)
	}
	return nil
}

func cmdPull(args []string) error {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	project := fs.String("project", "", "project id")
	dir := fs.String("dir", "./locales", "output directory")
	state := fs.String("state", "", "only export translations in this state (e.g. reviewed)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	pid, err := resolveProject(*project, cfg)
	if err != nil {
		return err
	}
	c := newClient(cfg)

	langs, err := c.listLanguages(pid)
	if err != nil {
		return err
	}
	feed, err := c.editorFeed(pid, 500)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return err
	}

	for _, l := range langs {
		out := map[string]string{}
		for _, row := range feed.Keys {
			t, ok := row.Translations[l.ID]
			if !ok || strings.TrimSpace(t.Text) == "" {
				continue
			}
			if *state != "" && t.State != *state {
				continue
			}
			out[row.Name] = t.Text
		}
		// json.Marshal sorts map keys, so output is deterministic.
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		path := filepath.Join(*dir, l.Tag+".json")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s (%d strings)\n", path, len(out))
	}
	return nil
}

func cmdPush(args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	project := fs.String("project", "", "project id")
	dir := fs.String("dir", "./locales", "input directory")
	lang := fs.String("lang", "", "language tag to push (e.g. fr)")
	dryRun := fs.Bool("dry-run", false, "show what would change without writing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *lang == "" {
		return fmt.Errorf("--lang is required")
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	pid, err := resolveProject(*project, cfg)
	if err != nil {
		return err
	}
	c := newClient(cfg)

	raw, err := os.ReadFile(filepath.Join(*dir, *lang+".json"))
	if err != nil {
		return err
	}
	var incoming map[string]string
	if err := json.Unmarshal(raw, &incoming); err != nil {
		return fmt.Errorf("parsing %s.json: %w", *lang, err)
	}

	feed, err := c.editorFeed(pid, 500)
	if err != nil {
		return err
	}
	byName := map[string]EditorRow{}
	for _, row := range feed.Keys {
		byName[row.Name] = row
	}

	// Resolve the language id for the existing-text comparison.
	langs, err := c.listLanguages(pid)
	if err != nil {
		return err
	}
	var langID string
	for _, l := range langs {
		if l.Tag == *lang {
			langID = l.ID
		}
	}
	if langID == "" {
		return fmt.Errorf("no language %q in this project", *lang)
	}

	created, updated, unchanged := 0, 0, 0
	names := make([]string, 0, len(incoming))
	for k := range incoming {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		text := incoming[name]
		row, exists := byName[name]
		switch {
		case !exists:
			created++
			if *dryRun {
				fmt.Printf("+ %s = %q (new key)\n", name, text)
				continue
			}
			key, err := c.createKey(pid, name)
			if err != nil {
				return fmt.Errorf("create key %q: %w", name, err)
			}
			if _, err := c.setTranslation(pid, key.ID, *lang, text); err != nil {
				return fmt.Errorf("set %q: %w", name, err)
			}
		case row.Translations[langID].Text != text:
			updated++
			if *dryRun {
				fmt.Printf("~ %s = %q (was %q)\n", name, text, row.Translations[langID].Text)
				continue
			}
			if _, err := c.setTranslation(pid, row.ID, *lang, text); err != nil {
				return fmt.Errorf("set %q: %w", name, err)
			}
		default:
			unchanged++
		}
	}
	verb := "applied"
	if *dryRun {
		verb = "would apply"
	}
	fmt.Printf("%s: %d new, %d updated, %d unchanged\n", verb, created, updated, unchanged)
	return nil
}

var extractPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bt\(\s*['"` + "`" + `]([^'"` + "`" + `]+)['"` + "`" + `]`), // t('key')
	regexp.MustCompile(`translationKey\s*=\s*['"]([^'"]+)['"]`),                      // <T translationKey="key">
}

func cmdExtract(args []string) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	project := fs.String("project", "", "project id (for --check)")
	check := fs.Bool("check", false, "fail if any used key is missing on the server")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// Go's flag package stops at the first positional, so flags placed after the
	// path (e.g. `extract ./src --check`) would be ignored. Take the path, then
	// re-parse anything that followed it.
	root := fs.Arg(0)
	if rest := fs.Args(); len(rest) > 1 {
		if err := fs.Parse(rest[1:]); err != nil {
			return err
		}
	}
	if root == "" {
		root = "."
	}

	found := map[string]bool{}
	exts := map[string]bool{".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".svelte": true}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" || d.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if !exts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, re := range extractPatterns {
			for _, m := range re.FindAllStringSubmatch(string(data), -1) {
				found[m[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(found))
	for k := range found {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Println(k)
	}
	fmt.Fprintf(os.Stderr, "%d unique keys in %s\n", len(keys), root)

	if !*check {
		return nil
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	pid, err := resolveProject(*project, cfg)
	if err != nil {
		return err
	}
	feed, err := newClient(cfg).editorFeed(pid, 500)
	if err != nil {
		return err
	}
	remote := map[string]bool{}
	for _, row := range feed.Keys {
		remote[row.Name] = true
	}
	var missing []string
	for _, k := range keys {
		if !remote[k] {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%d key(s) used in code but missing on the server: %s", len(missing), strings.Join(missing, ", "))
	}
	fmt.Fprintln(os.Stderr, "all used keys exist on the server")
	return nil
}
