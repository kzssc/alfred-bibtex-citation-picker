package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

// BookOrbit search: the `c` keyword, and the results shown among the BibTeX
// results of the citation picker.

const (
	boTimeout   = 8 * time.Second
	boMinQuery  = 2 // BookOrbit rejects shorter queries
	boLimit     = 8 // per category; API max is 10
	boCacheTTL  = 5 * time.Minute
	boRerun     = 0.1
	boDebounce  = 150 * time.Millisecond
	bibtexFirst = 5 // BookOrbit results go after this many BibTeX results
)

// Modifiers wired to citation actions in the citation picker; blocked for BookOrbit items.
var citationMods = []string{"cmd", "alt", "shift", "fn", "cmd+shift", "alt+cmd", "ctrl+alt",
	"ctrl+alt+cmd", "shift+ctrl+alt+cmd", "fn+cmd"}

type boConfigError struct{ msg string }

func (e boConfigError) Error() string { return e.msg }

type boAuthError struct{ msg string }

func (e boAuthError) Error() string { return e.msg }

func optionalArg(args []string, i int) string {
	if len(args) > i {
		return args[i]
	}
	return ""
}

// ---------------------------------------------------------------------------
// Config / tokens

func boBaseURL() (string, error) {
	u := strings.TrimSpace(os.Getenv("BOOKORBIT_URL"))
	if u == "" {
		return "", boConfigError{"BOOKORBIT_URL is not set. Configure the workflow in Alfred Preferences."}
	}
	if !strings.Contains(u, "://") {
		u = "https://" + u
	}
	return strings.TrimRight(u, "/"), nil
}

type boTokens struct {
	URL                  string `json:"url"`
	Username             string `json:"username"`
	AccessToken          string `json:"accessToken"`
	AccessTokenExpiresAt string `json:"accessTokenExpiresAt,omitempty"`
	RefreshToken         string `json:"refreshToken"`
}

type boClient struct {
	base, username, password string
	http                     *http.Client
	mu                       sync.Mutex
	tokens                   *boTokens
}

func newBoClient() (*boClient, error) {
	base, err := boBaseURL()
	if err != nil {
		return nil, err
	}
	return &boClient{
		base:     base,
		username: strings.TrimSpace(os.Getenv("BOOKORBIT_USERNAME")),
		password: os.Getenv("BOOKORBIT_PASSWORD"),
		http:     &http.Client{Timeout: boTimeout},
	}, nil
}

func boTokenPath() string {
	dir := os.Getenv("alfred_workflow_data")
	if dir == "" {
		home, _ := os.UserHomeDir()
		if home != "" {
			dir = filepath.Join(home, "Library/Application Support/Alfred/Workflow Data/uk.czhang.supercharged-citation-picker")
		} else {
			dir = filepath.Join(os.TempDir(), "alfred-citation-picker-data")
		}
	}
	os.MkdirAll(dir, 0o700)
	return filepath.Join(dir, "tokens.json")
}

func (c *boClient) loadTokens() *boTokens {
	data, err := os.ReadFile(boTokenPath())
	if err != nil {
		return nil
	}
	var t boTokens
	// tokens belong to one server/user; drop them if the config changed
	if json.Unmarshal(data, &t) != nil || t.URL != c.base || t.Username != c.username || t.AccessToken == "" {
		return nil
	}
	return &t
}

func (c *boClient) saveTokens(creds map[string]any) (*boTokens, error) {
	access, _ := creds["accessToken"].(string)
	refresh, _ := creds["refreshToken"].(string)
	if access == "" {
		return nil, errors.New("BookOrbit returned no access token")
	}
	expires, _ := creds["accessTokenExpiresAt"].(string)
	t := &boTokens{URL: c.base, Username: c.username, AccessToken: access, AccessTokenExpiresAt: expires, RefreshToken: refresh}
	data, _ := json.Marshal(t)
	path := boTokenPath()
	if err := os.WriteFile(path+".tmp", data, 0o600); err == nil {
		os.Rename(path+".tmp", path)
	}
	return t, nil
}

// ---------------------------------------------------------------------------
// HTTP / auth

func (c *boClient) request(method, path string, body any, token string) (map[string]any, error) {
	var reader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, c.base+"/api/v1"+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		uErr := errors.Unwrap(err)
		if uErr == nil {
			uErr = err
		}
		return nil, fmt.Errorf("Cannot reach %s: %v", c.base, uErr)
	}
	defer resp.Body.Close()
	var result map[string]any
	decodeErr := json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode >= 400 {
		message := http.StatusText(resp.StatusCode)
		switch m := result["message"].(type) {
		case string:
			message = m
		case []any:
			parts := make([]string, len(m))
			for i, p := range m {
				parts[i] = fmt.Sprint(p)
			}
			message = strings.Join(parts, "; ")
		}
		if resp.StatusCode == 401 {
			return nil, boAuthError{message}
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, message)
	}
	if decodeErr != nil {
		return map[string]any{}, nil
	}
	return result, nil
}

func (c *boClient) login() (*boTokens, error) {
	if c.username == "" || c.password == "" {
		return nil, boConfigError{"Username/password not set. Configure the workflow in Alfred Preferences."}
	}
	creds, err := c.request("POST", "/auth/login", map[string]string{
		"username":    c.username,
		"password":    c.password,
		"clientKind":  "native",
		"deviceLabel": "Alfred",
	}, "")
	if errors.As(err, new(boAuthError)) {
		return nil, boConfigError{"Login failed: check username and password in the workflow configuration."}
	}
	if err != nil {
		return nil, err
	}
	return c.saveTokens(creds)
}

func (c *boClient) refresh(t *boTokens) (*boTokens, error) {
	if t != nil && t.RefreshToken != "" {
		creds, err := c.request("POST", "/auth/refresh", map[string]string{"refreshToken": t.RefreshToken}, "")
		if err == nil {
			if nt, err := c.saveTokens(creds); err == nil {
				return nt, nil
			}
		}
	}
	return c.login()
}

// current access token, refreshed if about to expire
func (c *boClient) accessToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokens == nil {
		c.tokens = c.loadTokens()
	}
	var err error
	if c.tokens == nil {
		if c.tokens, err = c.login(); err != nil {
			return "", err
		}
	} else if exp, perr := time.Parse(time.RFC3339, c.tokens.AccessTokenExpiresAt); perr == nil && time.Until(exp) < 30*time.Second {
		if c.tokens, err = c.refresh(c.tokens); err != nil {
			return "", err
		}
	}
	return c.tokens.AccessToken, nil
}

// access token revoked or expired early: refresh (or re-login) once, unless
// another request already did
func (c *boClient) renewToken(stale string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokens != nil && c.tokens.AccessToken != stale {
		return c.tokens.AccessToken, nil
	}
	t, err := c.refresh(c.tokens)
	if err != nil {
		return "", err
	}
	c.tokens = t
	return t.AccessToken, nil
}

func (c *boClient) authedGet(path string) (map[string]any, error) {
	token, err := c.accessToken()
	if err != nil {
		return nil, err
	}
	result, err := c.request("GET", path, nil, token)
	if errors.As(err, new(boAuthError)) {
		if token, err = c.renewToken(token); err != nil {
			return nil, err
		}
		return c.request("GET", path, nil, token)
	}
	return result, err
}

// ---------------------------------------------------------------------------
// Search

type boHit struct {
	kind string // books, series, authors
	item map[string]any
}

func (h boHit) key() string { return h.kind + "/" + fmt.Sprint(h.item["id"]) }

func (c *boClient) fetch(query string, limit int) ([]boHit, error) {
	result, err := c.authedGet("/search?" + url.Values{"q": {query}, "limit": {fmt.Sprint(limit)}}.Encode())
	if err != nil {
		return nil, err
	}
	var hits []boHit
	for _, kind := range []string{"books", "series", "authors"} {
		group, _ := result[kind].(map[string]any)
		items, _ := group["items"].([]any)
		for _, h := range items {
			if item, ok := h.(map[string]any)["item"].(map[string]any); ok {
				hits = append(hits, boHit{kind, item})
			}
		}
	}
	return hits, nil
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func strList(v any) []string {
	list, _ := v.([]any)
	var out []string
	for _, x := range list {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (h boHit) words() []string {
	fields := []string{str(h.item["title"]), str(h.item["subtitle"]), str(h.item["name"]), str(h.item["seriesName"])}
	fields = append(fields, strList(h.item["authors"])...)
	return normalizeWords(strings.Join(fields, " "))
}

// BookOrbit matches the query as one phrase against the title or the subtitle
// separately, so a pasted "Title: Subtitle" (or words in another order) finds
// nothing. So the parts of the title and the longest words are searched as
// well, in parallel, and used in that order of preference when the full query
// has no results. Hits from single words must contain every query word.
var boSeparators = regexp.MustCompile(`\s*[:;|/]\s*|\s+[-\x{2013}\x{2014}]\s+|\s*\x{2014}\s*`)

func (c *boClient) fetchLenient(query string) ([]boHit, error) {
	var parts []string
	for _, part := range boSeparators.Split(query, -1) {
		part = strings.TrimSpace(part)
		if utf8.RuneCountInString(part) >= boMinQuery && part != query {
			parts = append(parts, part)
		}
	}
	queryWords := normalizeWords(query)
	var longest []string
	if len(queryWords) >= 2 {
		seen := map[string]bool{}
		for _, w := range queryWords {
			if !seen[w] && utf8.RuneCountInString(w) >= 3 {
				seen[w] = true
				longest = append(longest, w)
			}
		}
		sort.SliceStable(longest, func(i, j int) bool {
			return utf8.RuneCountInString(longest[i]) > utf8.RuneCountInString(longest[j])
		})
		if len(longest) > 3 {
			longest = longest[:3]
		}
	}

	type result struct {
		hits []boHit
		err  error
	}
	queries := append(append([]string{query}, parts...), longest...)
	results := make([]result, len(queries))
	var wg sync.WaitGroup
	for i, q := range queries {
		limit := boLimit
		if i > len(parts) {
			limit = 10 // single words: fetch more, most are filtered out
		}
		wg.Add(1)
		go func(i int, q string, limit int) {
			defer wg.Done()
			hits, err := c.fetch(q, limit)
			results[i] = result{hits, err}
		}(i, q, limit)
	}
	wg.Wait()

	if results[0].err != nil {
		return nil, results[0].err
	}
	if len(results[0].hits) > 0 {
		return results[0].hits, nil
	}

	var hits []boHit
	seen := map[string]bool{}
	for _, r := range results[1 : 1+len(parts)] {
		for _, h := range r.hits {
			if !seen[h.key()] {
				seen[h.key()] = true
				hits = append(hits, h)
			}
		}
	}
	if len(hits) > 0 {
		return hits, nil
	}

	for _, r := range results[1+len(parts):] {
		for _, h := range r.hits {
			if matchesQuery(h.words(), queryWords) {
				hits = append(hits, h)
			}
		}
		if len(hits) > 0 {
			return hits, nil
		}
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// Alfred items

type alfredMap = map[string]any

func boMessage(title, subtitle string, valid bool, arg string) alfredMap {
	item := alfredMap{"title": title, "subtitle": subtitle, "valid": valid}
	if arg != "" {
		item["arg"] = arg
	}
	return item
}

func boLinkItem(title, subtitle, link, uid string) alfredMap {
	return alfredMap{
		"uid":          uid,
		"title":        title,
		"subtitle":     subtitle,
		"arg":          link,
		"quicklookurl": link,
		"text":         alfredMap{"copy": link, "largetype": title},
		"mods":         alfredMap{"cmd": alfredMap{"subtitle": "Copy link: " + link, "arg": link}},
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func num(v any) int {
	f, _ := v.(float64)
	return int(f)
}

func boItem(h boHit, base string) alfredMap {
	id := fmt.Sprint(h.item["id"])
	switch h.kind {
	case "books":
		var parts []string
		if authors := strList(h.item["authors"]); len(authors) > 0 {
			parts = append(parts, strings.Join(authors, ", "))
		}
		if series := str(h.item["seriesName"]); series != "" {
			if idx := h.item["seriesIndex"]; idx != nil && fmt.Sprint(idx) != "" {
				series += " #" + fmt.Sprint(idx)
			}
			parts = append(parts, series)
		}
		if year := h.item["publishedYear"]; year != nil {
			parts = append(parts, fmt.Sprint(num(year)))
		}
		title := str(h.item["title"])
		if title == "" {
			title = "(untitled)"
		}
		if sub := str(h.item["subtitle"]); sub != "" {
			title += ": " + sub
		}
		subtitle := strings.Join(parts, " · ")
		if subtitle == "" {
			subtitle = "Book"
		}
		return boLinkItem("📖 "+title, subtitle, base+"/book/"+id, "book-"+id)
	case "series":
		parts := []string{"Series"}
		if authors := strList(h.item["authors"]); len(authors) > 0 {
			parts = append(parts, strings.Join(authors, ", "))
		}
		parts = append(parts, plural(num(h.item["bookCount"]), "book"))
		return boLinkItem("📚 "+str(h.item["name"]), strings.Join(parts, " · "), base+"/series/"+id, "series-"+id)
	default:
		return boLinkItem("👤 "+str(h.item["name"]), "Author · "+plural(num(h.item["bookCount"]), "book"),
			base+"/authors/"+id, "author-"+id)
	}
}

// returns the items, and whether they are actual results
func boSearch(query string) ([]alfredMap, bool) {
	query = strings.TrimSpace(query)
	client, err := newBoClient()
	if err != nil {
		return []alfredMap{boMessage("BookOrbit is not configured", err.Error(), false, "")}, false
	}
	if utf8.RuneCountInString(query) < boMinQuery {
		return []alfredMap{boMessage("Search BookOrbit", fmt.Sprintf("Type at least %d characters · ↩ open BookOrbit", boMinQuery), true, client.base)}, false
	}
	hits, err := client.fetchLenient(query)
	if errors.As(err, new(boConfigError)) {
		return []alfredMap{boMessage("BookOrbit is not configured", err.Error(), false, "")}, false
	}
	if err != nil {
		return []alfredMap{boMessage("BookOrbit error", err.Error(), false, "")}, false
	}
	if len(hits) == 0 {
		return []alfredMap{boMessage("No results for “"+query+"”", "↩ open BookOrbit", true, client.base)}, false
	}
	items := make([]alfredMap, len(hits))
	for i, h := range hits {
		items[i] = boItem(h, client.base)
	}
	return items, true
}

// adapt items for display among the citation picker's BibTeX results
func forCitationPicker(items []alfredMap) []alfredMap {
	for _, item := range items {
		if sub := str(item["subtitle"]); sub != "" {
			item["subtitle"] = "BookOrbit · " + sub
		} else {
			item["subtitle"] = "BookOrbit"
		}
		item["icon"] = alfredMap{"path": "icons/bookorbit.png"}
		item["variables"] = alfredMap{"result_source": "bookorbit"}
		blocked := alfredMap{"valid": false, "subtitle": "BookOrbit result · ↩ open · ⌃↩ open URL · ⌘C copy link"}
		mods := alfredMap{}
		for _, mod := range citationMods {
			mods[mod] = blocked
		}
		mods["ctrl"] = blocked
		if arg := str(item["arg"]); arg != "" {
			mods["ctrl"] = alfredMap{"valid": true, "arg": arg, "subtitle": "Open " + arg}
		}
		item["mods"] = mods
	}
	return items
}

// `c` keyword
func searchBookOrbit(query string) {
	items, _ := boSearch(query)
	json.NewEncoder(os.Stdout).Encode(alfredMap{"items": items})
}

// ---------------------------------------------------------------------------
// Citation picker: BibTeX results come back instantly, BookOrbit needs the
// network. So BookOrbit is fetched in the background into a cache and the
// script filter reruns until it is there, showing BibTeX results meanwhile.

type boCached struct {
	Found bool        `json:"found"`
	Items []alfredMap `json:"items"`
}

func boCacheDir() string {
	dir := os.Getenv("alfred_workflow_cache")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "bookorbit-alfred")
	}
	dir = filepath.Join(dir, "bookorbit")
	os.MkdirAll(dir, 0o755)
	return dir
}

func boCacheFile(query string) string {
	sum := sha1.Sum([]byte(query))
	return filepath.Join(boCacheDir(), hex.EncodeToString(sum[:])+".json")
}

func boLatestFile() string { return filepath.Join(boCacheDir(), "latest") }

func fetchBookOrbitToCache(query string) {
	// debounce: while typing, only the latest query is fetched
	time.Sleep(boDebounce)
	path := boCacheFile(query)
	defer os.Remove(path + ".pending")
	if latest, err := os.ReadFile(boLatestFile()); err == nil && string(latest) != query {
		return
	}
	items, found := boSearch(query)
	data, _ := json.Marshal(boCached{found, forCitationPicker(items)})
	if os.WriteFile(path+".tmp", data, 0o644) == nil {
		os.Rename(path+".tmp", path)
	}
}

func boReadCache(query string) *boCached {
	path := boCacheFile(query)
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > boCacheTTL {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c boCached
	if json.Unmarshal(data, &c) != nil {
		return nil
	}
	return &c
}

func boStartFetch(query string) {
	os.WriteFile(boLatestFile(), []byte(query), 0o644)
	pending := boCacheFile(query) + ".pending"
	if info, err := os.Stat(pending); err == nil && time.Since(info.ModTime()) < 3*boTimeout {
		return // already being fetched
	}
	os.WriteFile(pending, nil, 0o644)
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, "bookorbit-fetch", query)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Start() == nil {
		cmd.Process.Release()
	}
}

func searchCitationPicker(bufferPath, query string) {
	bibtex, err := filterItems(bufferPath, query)
	if err != nil {
		bibtex = nil
	}
	out := struct {
		Items []any    `json:"items"`
		Rerun *float64 `json:"rerun,omitempty"`
	}{Items: []any{}}
	addBibtex := func(items []json.RawMessage) {
		for _, item := range items {
			out.Items = append(out.Items, item)
		}
	}
	split := min(bibtexFirst, len(bibtex))

	query = strings.TrimSpace(query)
	// BookOrbit is optional: only searched when a server is configured
	boEnabled := strings.TrimSpace(os.Getenv("BOOKORBIT_URL")) != ""
	switch {
	case !boEnabled || utf8.RuneCountInString(query) < boMinQuery:
		addBibtex(bibtex)
		if len(out.Items) == 0 && query == "" {
			out.Items = append(out.Items, boMessage("Library is empty", "", false, ""))
		}
	default:
		cached := boReadCache(query)
		addBibtex(bibtex[:split])
		if cached == nil {
			boStartFetch(query)
			status := boMessage("Searching BookOrbit…", query, false, "")
			status["icon"] = alfredMap{"path": "icons/bookorbit.png"}
			out.Items = append(out.Items, status)
			rerun := boRerun
			out.Rerun = &rerun
		} else {
			os.Remove(boCacheFile(query) + ".pending")
			for _, item := range cached.Items {
				// keep errors visible, but drop "no results" when BibTeX has matches
				if len(bibtex) > 0 && !cached.Found && strings.HasPrefix(str(item["title"]), "No results") {
					continue
				}
				out.Items = append(out.Items, item)
			}
		}
		addBibtex(bibtex[split:])
	}
	json.NewEncoder(os.Stdout).Encode(out)
}
