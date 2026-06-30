package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type BibtexEntry struct {
	Author     []string
	Editor     []string
	Icon       string
	Citekey    string
	Title      string
	Year       string
	OrigYear   string
	URL        string
	BookTitle  string
	Journal    string
	DOI        string
	Volume     string
	Issue      string
	Abstract   string
	Keywords   []string
	Attachment string
}

var bibtexReplacer = strings.NewReplacer(
	`{\\"u}`, "ü",
	`{\\"a}`, "ä",
	`{\\"o}`, "ö",
	`{\\"U}`, "Ü",
	`{\\"A}`, "Ä",
	`{\\"O}`, "Ö",
	`\\"u`, "ü",
	`\\"a`, "ä",
	`\\"o`, "ö",
	`\\"U`, "Ü",
	`\\"A`, "Ä",
	`\\"O`, "Ö",
	`\ss`, "ß",
	`{\ss}`, "ß",
	`\\"{O}`, "Ö",
	`\\"{o}`, "ö",
	`\\"{A}`, "Ä",
	`\\"{a}`, "ä",
	`\\"{u}`, "ü",
	`\\"{U}`, "Ü",
	`\\''A`, "Ä",
	`\\''O`, "Ö",
	`\\''U`, "Ü",
	`\\''a`, "ä",
	`\\''o`, "ö",
	`\\''u`, "ü",
	`{\\'a}`, "a",
	`{\\'o}`, "ó",
	`{\\'e}`, "e",
	"{\x60{e}}", "e",
	"{\x60e}", "e",
	`\\'E`, "É",
	`\c{c}`, "c",
	`\\"{i}`, "i",
	`{\~n}`, "n",
	`\~a`, "ã",
	`{\v c}`, "c",
	`\o{}`, "ø",
	`{\o}`, "ø",
	`{\O}`, "Ø",
	`\^{i}`, "i",
	`\'\i`, "í",
	`{\'c}`, "c",
	`{\ldots}`, "…",
	`\&`, "&",
	"``", "\"",
	",,", "\"",
	"`", "'",
	`\textendash{}`, "—",
	"---", "—",
	"--", "—",
	"{extquotesingle}", "'",
	`\\"e`, "e",
)

func bibtexDecode(s string) string {
	return bibtexReplacer.Replace(s)
}

func splitProperties(s string) []string {
	var parts []string
	var start = 0
	var braceDepth = 0
	var inQuotes = false

	for i := 0; i < len(s); i++ {
		char := s[i]
		if char == '{' {
			braceDepth++
		} else if char == '}' {
			braceDepth--
		} else if char == '"' {
			inQuotes = !inQuotes
		} else if char == ',' && braceDepth == 0 && !inQuotes {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

var yearRegex = regexp.MustCompile(`\d{4}`)

func bibtexParse(raw string) []BibtexEntry {
	decoded := bibtexDecode(raw)
	entriesRaw := strings.Split("\n"+decoded, "\n@")
	if len(entriesRaw) > 0 {
		entriesRaw = entriesRaw[1:]
	}

	entries := make([]BibtexEntry, 0, len(entriesRaw))

	for _, rawEntry := range entriesRaw {
		trimmed := strings.TrimSpace(rawEntry)
		openBracePos := strings.Index(trimmed, "{")
		if openBracePos == -1 {
			continue
		}
		firstCommaPos := strings.Index(trimmed[openBracePos:], ",")
		if firstCommaPos == -1 {
			continue
		}
		firstCommaPos += openBracePos

		category := strings.TrimSpace(trimmed[:openBracePos])
		citekey := strings.TrimSpace(trimmed[openBracePos+1 : firstCommaPos])
		propertyStr := trimmed[firstCommaPos+1:]
		if strings.HasSuffix(propertyStr, "}") {
			propertyStr = propertyStr[:len(propertyStr)-1]
		}

		entry := BibtexEntry{
			Citekey: citekey,
			Icon:    strings.ToLower(category),
		}

		properties := splitProperties(propertyStr)
		for _, line := range properties {
			equalSignPos := strings.Index(line, "=")
			if equalSignPos == -1 {
				continue
			}

			field := strings.ToLower(strings.TrimSpace(line[:equalSignPos]))
			value := strings.TrimSpace(line[equalSignPos+1:])
			value = strings.Trim(value, "{},")

			switch field {
			case "author", "editor":
				names := strings.Split(value, " and ")
				parsedNames := make([]string, len(names))
				for idx, name := range names {
					name = strings.TrimSpace(name)
					var lastname string
					if strings.Contains(name, ",") {
						lastname = strings.Split(name, ",")[0]
					} else {
						parts := strings.Fields(name)
						if len(parts) > 0 {
							lastname = parts[len(parts)-1]
						}
					}
					if lastname == "" {
						lastname = "ERROR"
					}
					parsedNames[idx] = lastname
				}
				if field == "author" {
					entry.Author = parsedNames
				} else {
					entry.Editor = parsedNames
				}
			case "date", "year":
				match := yearRegex.FindString(value)
				if match != "" {
					entry.Year = match
				}
			case "keywords":
				if value != "" {
					kws := strings.Split(value, ",")
					for idx, kw := range kws {
						kws[idx] = strings.Trim(strings.TrimSpace(kw), "{}")
					}
					entry.Keywords = kws
				}
			case "file", "local-url", "attachment":
				entry.Attachment = value
			case "title":
				entry.Title = value
			case "origyear":
				entry.OrigYear = value
			case "url":
				entry.URL = value
			case "booktitle":
				entry.BookTitle = value
			case "journal":
				entry.Journal = value
			case "doi":
				entry.DOI = value
			case "volume":
				entry.Volume = value
			case "issue":
				entry.Issue = value
			case "abstract":
				entry.Abstract = value
			}
		}

		if entry.URL == "" && entry.DOI != "" {
			entry.URL = "https://doi.org/" + entry.DOI
		}
		entries = append(entries, entry)
	}

	return entries
}

type AlfredIcon struct {
	Path string `json:"path"`
}

type AlfredText struct {
	Copy      string `json:"copy,omitempty"`
	LargeType string `json:"largetype,omitempty"`
}

type AlfredMod struct {
	Valid    bool   `json:"valid"`
	Arg      string `json:"arg,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
}

type AlfredMods struct {
	Ctrl       *AlfredMod `json:"ctrl,omitempty"`
	Shift      *AlfredMod `json:"shift,omitempty"`
	FnCmd      *AlfredMod `json:"fn+cmd,omitempty"`
	CtrlAltCmd *AlfredMod `json:"ctrl+alt+cmd,omitempty"`
}

type AlfredItem struct {
	Title        string      `json:"title"`
	Autocomplete string      `json:"autocomplete,omitempty"`
	Subtitle     string      `json:"subtitle"`
	Match        string      `json:"match"`
	Arg          string      `json:"arg"`
	Icon         *AlfredIcon `json:"icon,omitempty"`
	UID          string      `json:"uid,omitempty"`
	Text         *AlfredText `json:"text,omitempty"`
	QuicklookURL string      `json:"quicklookurl,omitempty"`
	Mods         *AlfredMods `json:"mods,omitempty"`
}

type AlfredResponse struct {
	Items []AlfredItem `json:"items"`
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func dirExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func loadListFile(path string) map[string]bool {
	m := make(map[string]bool)
	if !fileExists(path) {
		return m
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			m[trimmed] = true
		}
	}
	return m
}

func convertToAlfredItem(entry BibtexEntry, whichLibrary string, alfredBarWidth int, openEntriesIn string, matchAuthorsInEtAl bool, matchShortYears bool, matchFullYears bool, litNoteSet map[string]bool, pdfSet map[string]bool) AlfredItem {
	isFirstLibrary := whichLibrary == "first"

	// Shorten title
	title := entry.Title
	if len(title) > alfredBarWidth {
		runes := []rune(title)
		if len(runes) > alfredBarWidth {
			title = string(runes[:alfredBarWidth]) + "…"
		}
	}

	// Emojis
	var emojis []string

	if entry.URL != "" {
		emojis = append(emojis, "🌐")
	}

	var extraMatcher string
	if litNoteSet[entry.Citekey] {
		emojis = append(emojis, "📓")
		extraMatcher += "*"
	}

	if pdfSet[entry.Citekey] {
		emojis = append(emojis, "📕")
		extraMatcher += "pdf"
	}

	if entry.Abstract != "" {
		emojis = append(emojis, "📄")
	}
	if len(entry.Keywords) > 0 {
		emojis = append(emojis, fmt.Sprintf("🏷 %d", len(entry.Keywords)))
	}
	if entry.Attachment != "" {
		emojis = append(emojis, "📎")
	}

	libraryIndicator := ""
	if whichLibrary == "second" {
		libraryIndicator = "2️⃣ "
	}

	var primaryNames []string
	if len(entry.Author) > 0 {
		primaryNames = entry.Author
	} else {
		primaryNames = entry.Editor
	}

	var etAlString string
	switch len(primaryNames) {
	case 0:
		etAlString = ""
	case 1:
		etAlString = primaryNames[0]
	case 2:
		etAlString = primaryNames[0] + " & " + primaryNames[1]
	default:
		etAlString = primaryNames[0] + " et al."
	}

	namesToDisplay := etAlString + " "
	if len(entry.Author) == 0 && len(entry.Editor) > 0 {
		if len(entry.Editor) > 1 {
			namesToDisplay += "(Eds.) "
		} else {
			namesToDisplay += "(Ed.) "
		}
	}

	displayYear := entry.Year
	if entry.OrigYear != "" {
		displayYear = fmt.Sprintf("%s [%s]", entry.Year, entry.OrigYear)
	}

	var collectionSubtitle string
	if entry.Icon == "article" && entry.Journal != "" {
		collectionSubtitle += "    In: " + entry.Journal
		if entry.Volume != "" {
			collectionSubtitle += " " + entry.Volume
		}
		if entry.Issue != "" {
			collectionSubtitle += "(" + entry.Issue + ")"
		}
	}
	if (entry.Icon == "incollection" || entry.Icon == "inbook") && entry.BookTitle != "" {
		collectionSubtitle += "    In: " + entry.BookTitle
	}

	subtitle := fmt.Sprintf("%s%s%s", namesToDisplay, displayYear, collectionSubtitle)
	if len(emojis) > 0 {
		subtitle += "   " + strings.Join(emojis, " ")
	}

	var matcherParts []string
	matcherParts = append(matcherParts, "@"+entry.Citekey)
	for _, kw := range entry.Keywords {
		matcherParts = append(matcherParts, "#"+kw)
	}
	matcherParts = append(matcherParts, entry.Title)

	var authorMatches []string
	if matchAuthorsInEtAl {
		authorMatches = append(authorMatches, entry.Author...)
		authorMatches = append(authorMatches, entry.Editor...)
	} else {
		if len(entry.Author) > 0 {
			authorMatches = append(authorMatches, entry.Author[0])
		}
		if len(entry.Editor) > 0 {
			authorMatches = append(authorMatches, entry.Editor[0])
		}
	}
	matcherParts = append(matcherParts, authorMatches...)

	if matchShortYears && len(entry.Year) >= 2 {
		matcherParts = append(matcherParts, entry.Year[len(entry.Year)-2:])
	}
	if matchFullYears {
		matcherParts = append(matcherParts, entry.Year)
	}
	if entry.BookTitle != "" {
		matcherParts = append(matcherParts, entry.BookTitle)
	}
	if entry.Journal != "" {
		matcherParts = append(matcherParts, entry.Journal)
	}
	if extraMatcher != "" {
		matcherParts = append(matcherParts, extraMatcher)
	}

	var finalMatcherParts []string
	for _, part := range matcherParts {
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			finalMatcherParts = append(finalMatcherParts, strings.ReplaceAll(part, "-", " ")+" "+part)
		} else {
			finalMatcherParts = append(finalMatcherParts, part)
		}
	}
	alfredMatcher := strings.Join(finalMatcherParts, " ")

	var autocomplete string
	if len(primaryNames) > 0 {
		autocomplete = primaryNames[0]
	}

	largeTypeInfo := fmt.Sprintf("%s \n(citekey: %s)", entry.Title, entry.Citekey)
	if entry.Abstract != "" {
		largeTypeInfo += "\n\n" + entry.Abstract
	}
	if len(entry.Keywords) > 0 {
		largeTypeInfo += "\n\nkeywords: " + strings.Join(entry.Keywords, ", ")
	}

	var ctrlSub string
	if entry.URL != "" {
		ctrlSub = "⌃: Open URL – " + entry.URL
	} else {
		ctrlSub = "⛔ There is no URL or DOI."
	}

	var shiftSub string
	if isFirstLibrary {
		shiftSub = "⇧: Open in " + openEntriesIn
	} else {
		shiftSub = "⛔: Opening entries in 2nd library not yet implemented."
	}

	var fnCmdSub string
	if isFirstLibrary {
		fnCmdSub = "⌘+fn: Delete entry from BibTeX file (⚠️ irreversible)."
	} else {
		fnCmdSub = "⛔: Deleting entries in 2nd library not yet implemented."
	}

	var ctrlAltCmdSub string
	if entry.Attachment != "" {
		ctrlAltCmdSub = "⌃⌥⌘: Open Attachment File"
	} else {
		ctrlAltCmdSub = "⛔: Entry has no attachment file."
	}

	mods := &AlfredMods{
		Ctrl: &AlfredMod{
			Valid:    entry.URL != "",
			Arg:      entry.URL,
			Subtitle: ctrlSub,
		},
		Shift: &AlfredMod{
			Valid:    isFirstLibrary,
			Subtitle: shiftSub,
		},
		FnCmd: &AlfredMod{
			Valid:    isFirstLibrary,
			Subtitle: fnCmdSub,
		},
		CtrlAltCmd: &AlfredMod{
			Valid:    entry.Attachment != "",
			Subtitle: ctrlAltCmdSub,
			Arg:      entry.Attachment,
		},
	}

	return AlfredItem{
		Title:        libraryIndicator + title,
		Autocomplete: autocomplete,
		Subtitle:     subtitle,
		Match:        alfredMatcher,
		Arg:          entry.Citekey,
		Icon: &AlfredIcon{
			Path: "icons/" + entry.Icon + ".png",
		},
		UID:          entry.Citekey,
		Text: &AlfredText{
			Copy:      entry.URL,
			LargeType: largeTypeInfo,
		},
		QuicklookURL: entry.URL,
		Mods:         mods,
	}
}

type ObsidianConfig struct {
	Vaults map[string]struct {
		Path string `json:"path"`
	} `json:"vaults"`
}

func openAttachment(rawPath string) {
	path := rawPath

	decoded, err := url.QueryUnescape(path)
	if err == nil {
		path = decoded
	}

	semicolonPos := strings.Index(path, ";/Users/")
	if semicolonPos != -1 {
		path = path[:semicolonPos]
	}

	path = strings.TrimPrefix(path, "file://localhost")
	path = strings.TrimPrefix(path, "file://")

	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}

	if !fileExists(path) && !dirExists(path) {
		fmt.Printf("File does not exist: %s\n", path)
		return
	}

	zoteroStorageRegex := regexp.MustCompile(`/Zotero/storage/([^/]{8})/`)
	match := zoteroStorageRegex.FindStringSubmatch(path)
	if len(match) > 1 {
		itemKey := match[1]
		cmd := exec.Command("open", "zotero://open-pdf/library/items/"+itemKey)
		cmd.Run()
		fmt.Printf("opening via Zotero scheme, itemKey: %s\n", itemKey)
	} else {
		cmd := exec.Command("open", path)
		cmd.Run()
		fmt.Printf("attachment path: %s\n", path)
	}
}

func openLitNote(citekey string) {
	litNoteFolder := os.Getenv("literature_note_folder")
	if strings.HasPrefix(litNoteFolder, "~") {
		home, _ := os.UserHomeDir()
		litNoteFolder = filepath.Join(home, litNoteFolder[1:])
	}

	notePath := filepath.Join(litNoteFolder, citekey+".md")

	if !fileExists(notePath) {
		template := "---\ntags: \naliases:\n---\n\n"
		err := os.WriteFile(notePath, []byte(template), 0644)
		if err != nil {
			fmt.Printf("Failed to create note: %v\n", err)
			return
		}
	}

	home, _ := os.UserHomeDir()
	obsidianConfigPath := filepath.Join(home, "Library", "Application Support", "obsidian", "obsidian.json")

	inVault := false
	if fileExists(obsidianConfigPath) {
		configData, err := os.ReadFile(obsidianConfigPath)
		if err == nil {
			var config ObsidianConfig
			err = json.Unmarshal(configData, &config)
			if err == nil {
				notePathLower := strings.ToLower(notePath)
				for _, vault := range config.Vaults {
					vPath := strings.ToLower(vault.Path)
					if vPath != "" && strings.HasPrefix(notePathLower, vPath) {
						inVault = true
						break
					}
				}
			}
		}
	}

	if inVault {
		encodedPath := url.QueryEscape(notePath)
		encodedPath = strings.ReplaceAll(encodedPath, "+", "%20")
		cmd := exec.Command("open", "obsidian://open?path="+encodedPath)
		cmd.Run()
	} else {
		cmd := exec.Command("open", notePath)
		cmd.Run()
	}
}

func extractEntry(citekey string) {
	libraryPath := os.Getenv("bibtex_library_path")
	secondaryLibraryPath := os.Getenv("secondary_library_path")

	extracted := extractFromSingleLibrary(libraryPath, citekey)
	if extracted == "" && secondaryLibraryPath != "" {
		extracted = extractFromSingleLibrary(secondaryLibraryPath, citekey)
	}

	if extracted != "" {
		fmt.Print(extracted)
	}
}

func extractFromSingleLibrary(libraryPath string, citekey string) string {
	if libraryPath == "" || citekey == "" || !fileExists(libraryPath) {
		return ""
	}
	data, err := os.ReadFile(libraryPath)
	if err != nil {
		return ""
	}

	keyBytes := []byte("{" + citekey + ",")
	keyIndex := bytes.Index(data, keyBytes)
	if keyIndex == -1 {
		dataLower := bytes.ToLower(data)
		keyIndex = bytes.Index(dataLower, bytes.ToLower(keyBytes))
		if keyIndex == -1 {
			return ""
		}
	}

	start := keyIndex
	for start > 0 && data[start] != '@' {
		start--
	}

	braceDepth := 0
	end := keyIndex
	for end < len(data) {
		char := data[end]
		if char == '{' {
			braceDepth++
		} else if char == '}' {
			braceDepth--
			if braceDepth == 0 {
				end++
				break
			}
		}
		end++
	}

	if end > len(data) {
		end = len(data)
	}

	return string(data[start:end])
}

func main() {
	args := os.Args
	if len(args) > 1 {
		subcmd := args[1]
		switch subcmd {
		case "open-attachment":
			if len(args) < 3 {
				fmt.Println("Usage: open-attachment <path>")
				return
			}
			openAttachment(args[2])
			return
		case "open-litnote":
			if len(args) < 3 {
				fmt.Println("Usage: open-litnote <citekey>")
				return
			}
			openLitNote(args[2])
			return
		case "extract-entry":
			if len(args) < 3 {
				fmt.Println("Usage: extract-entry <citekey>")
				return
			}
			extractEntry(args[2])
			return
		}
	}

	alfredBarWidth, _ := strconv.Atoi(os.Getenv("alfred_bar_width"))
	if alfredBarWidth == 0 {
		alfredBarWidth = 80
	}
	alfredWorkflowData := os.Getenv("alfred_workflow_data")

	matchAuthorsInEtAl := os.Getenv("match_authors_in_etal") == "1"
	matchShortYears := strings.Contains(os.Getenv("match_year_type"), "short")
	matchFullYears := strings.Contains(os.Getenv("match_year_type"), "full")
	openEntriesIn := os.Getenv("open_entries_in")

	libraryPath := os.Getenv("bibtex_library_path")
	secondaryLibraryPath := os.Getenv("secondary_library_path")

	litNoteFolder := os.Getenv("literature_note_folder")
	pdfFolder := os.Getenv("pdf_folder")

	litNoteFolderExists := dirExists(litNoteFolder)
	pdfFolderExists := dirExists(pdfFolder)

	// Guard checks matching JS
	if pdfFolder != "" && !pdfFolderExists {
		resp, _ := json.Marshal(AlfredResponse{
			Items: []AlfredItem{
				{Title: "PDF folder does not exist.", Subtitle: pdfFolder, Arg: "", Match: ""},
			},
		})
		fmt.Println(string(resp))
		return
	}
	if litNoteFolder != "" && !litNoteFolderExists {
		resp, _ := json.Marshal(AlfredResponse{
			Items: []AlfredItem{
				{Title: "Literature folder does not exist.", Subtitle: litNoteFolder, Arg: "", Match: ""},
			},
		})
		fmt.Println(string(resp))
		return
	}

	pdfListFile := filepath.Join(alfredWorkflowData, "pdf_list.txt")
	litListFile := filepath.Join(alfredWorkflowData, "lit_list.txt")

	var pdfSet map[string]bool
	if pdfFolderExists {
		pdfSet = loadListFile(pdfListFile)
	} else {
		pdfSet = make(map[string]bool)
	}

	var litNoteSet map[string]bool
	if litNoteFolderExists {
		litNoteSet = loadListFile(litListFile)
	} else {
		litNoteSet = make(map[string]bool)
	}

	// Parse first library
	var firstBibtexItems []AlfredItem
	if fileExists(libraryPath) {
		content, err := os.ReadFile(libraryPath)
		if err == nil {
			entries := bibtexParse(string(content))
			// Reverse so recent entries come first (mapping JS behavior)
			for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
				entries[i], entries[j] = entries[j], entries[i]
			}
			firstBibtexItems = make([]AlfredItem, len(entries))
			for idx, entry := range entries {
				firstBibtexItems[idx] = convertToAlfredItem(entry, "first", alfredBarWidth, openEntriesIn, matchAuthorsInEtAl, matchShortYears, matchFullYears, litNoteSet, pdfSet)
			}
		}
	}

	// Parse second library
	var secondBibtexItems []AlfredItem
	if fileExists(secondaryLibraryPath) {
		content, err := os.ReadFile(secondaryLibraryPath)
		if err == nil {
			entries := bibtexParse(string(content))
			// Reverse so recent entries come first
			for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
				entries[i], entries[j] = entries[j], entries[i]
			}
			secondBibtexItems = make([]AlfredItem, len(entries))
			for idx, entry := range entries {
				secondBibtexItems[idx] = convertToAlfredItem(entry, "second", alfredBarWidth, openEntriesIn, matchAuthorsInEtAl, matchShortYears, matchFullYears, litNoteSet, pdfSet)
			}
		}
	}

	allItems := append(firstBibtexItems, secondBibtexItems...)
	resp := AlfredResponse{Items: allItems}
	jsonBytes, err := json.Marshal(resp)
	if err == nil {
		fmt.Println(string(jsonBytes))
	}
}
