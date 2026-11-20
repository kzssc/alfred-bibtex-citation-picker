package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// Filtering is done here instead of by Alfred, so the script filter knows
// when the library has no match and can fall back to BookOrbit.
// Parsing the whole buffer per keystroke is slow, so a sidecar index
// (`<offset> <length> <normalized match words>` per item) is kept next to it.

const maxFilterResults = 100

// bump when the index format or word normalization changes
const indexVersion = "v2"

type indexEntry struct {
	offset, length int64
	words          []string
}

// Lowercased, diacritic-folded words split on anything that is not a letter or
// digit, so pasted titles match regardless of typographic punctuation
// (’ vs ', — vs - vs :, non-breaking spaces).
func normalizeWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(foldDiacritics(s)), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func buildIndex(bufferPath, indexPath string) error {
	data, err := os.ReadFile(bufferPath)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	// walk to the start of the "items" array
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if key, ok := tok.(string); ok && key == "items" {
			if _, err := dec.Token(); err != nil {
				return err
			}
			break
		}
	}
	tmp := indexPath + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, indexVersion)
	for dec.More() {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			f.Close()
			return err
		}
		end := dec.InputOffset()
		var item struct {
			Match string `json:"match"`
		}
		json.Unmarshal(raw, &item)
		fmt.Fprintf(w, "%d %d %s\n", end-int64(len(raw)), len(raw), strings.Join(normalizeWords(item.Match), " "))
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmp, indexPath)
}

func loadIndex(bufferPath string) ([]indexEntry, error) {
	indexPath := bufferPath + ".idx"
	bufInfo, err := os.Stat(bufferPath)
	if err != nil {
		return nil, err
	}
	if idxInfo, err := os.Stat(indexPath); err != nil || idxInfo.ModTime().Before(bufInfo.ModTime()) {
		if err := buildIndex(bufferPath, indexPath); err != nil {
			return nil, err
		}
	}
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(content), "\n")
	if lines[0] != indexVersion {
		if err := buildIndex(bufferPath, indexPath); err != nil {
			return nil, err
		}
		if content, err = os.ReadFile(indexPath); err != nil {
			return nil, err
		}
		lines = strings.Split(string(content), "\n")
	}
	var entries []indexEntry
	for _, line := range lines[1:] {
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}
		offset, _ := strconv.ParseInt(parts[0], 10, 64)
		length, _ := strconv.ParseInt(parts[1], 10, 64)
		var words []string
		if len(parts) == 3 {
			words = strings.Split(parts[2], " ")
		}
		entries = append(entries, indexEntry{offset, length, words})
	}
	return entries, nil
}

// every query word must be the prefix of some word of the item, in any order
func matchesQuery(words, queryWords []string) bool {
	for _, q := range queryWords {
		found := false
		for _, w := range words {
			if strings.HasPrefix(w, q) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// returns up to maxFilterResults matching items, as raw Alfred JSON
func filterItems(bufferPath, query string) ([]json.RawMessage, error) {
	entries, err := loadIndex(bufferPath)
	if err != nil {
		return nil, err
	}
	queryWords := normalizeWords(query)

	f, err := os.Open(bufferPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var items []json.RawMessage
	for _, e := range entries {
		if !matchesQuery(e.words, queryWords) {
			continue
		}
		raw := make([]byte, e.length)
		if _, err := f.ReadAt(raw, e.offset); err != nil {
			continue
		}
		items = append(items, raw)
		if len(items) >= maxFilterResults {
			break
		}
	}
	return items, nil
}
