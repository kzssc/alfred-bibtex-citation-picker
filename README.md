# Supercharged citation picker

A fast citation picker and lightweight reference manager for [Alfred 5](https://www.alfredapp.com/). Works with `.bib` files and supports Pandoc, LaTeX, Multi-Markdown, Org mode, eta templates, Obsidian wikilinks, and plain citekeys.

## About this fork
This repository is a fork of [chrisgrieser/alfred-bibtex-citation-picker](https://github.com/chrisgrieser/alfred-bibtex-citation-picker) focused on performance, search reliability, and broader ecosystem integration:
- **Go performance rewrite**: Cache building, single-citation formatting, and file opening run through a compiled universal binary (`arm64` and `x86_64`). Single-citation pasting is ~22x faster, and note or attachment opening is near-instant.
- **Punctuation and diacritic-insensitive search**: Filtering is performed natively in Go with unicode normalization and diacritic folding. Copied titles match regardless of colon, dash, or quote variations (`:`, `—`, `-`, `’`) or LaTeX accent macros.
- **Shift attachment preview**: Pressing <kbd>⇧</kbd> (Shift) on an entry displays the local attachment file (such as a PDF) directly in Quick Look. Entries without local attachments preview their URL or DOI.
- **Zotero protocol opening**: PDFs stored inside Zotero storage directories open via the `zotero://open-pdf/` URI scheme, synchronizing annotations, reading position, and read status with Zotero.
- **Unified BookOrbit search**: With a BookOrbit server configured, matching books, series, and authors appear directly alongside BibTeX results or through the dedicated `bo` keyword.

## Feature overview

### Core capabilities
- **System-wide operation**: Works in any macOS text field across all applications.
- **Flexible citation formats**: Inserts Pandoc (`[@citekey]`), LaTeX (`\autocite{citekey}`), Multi-Markdown, Org mode, eta templates, Obsidian `[[wikilinks]]`, `#tags`, or bare citekeys.
- **Multi-citation chaining**: Add multiple citations with page numbers in a single workflow invocation (e.g. `[@citekey1, p. 23; @citekey2, p. 42]`).
- **Dual library support**: Search across a primary and secondary `.bib` library simultaneously.
- **Instant response**: Pre-indexes library entries into a compact index so search results render immediately upon typing.

### Advanced capabilities
- **Formatted bibliography pasting**: Format and paste single references via Pandoc using APA 7th or any custom `.csl` style.
- **Literature notes**: Open or create Markdown literature notes matched to citekeys, with direct integration for Obsidian vaults.
- **PDF auto-filing**: Automatically rename and move downloaded PDFs into your reference library folder using citekey conventions.
- **Entry creation**: Add new entries to your BibTeX file directly from selected DOIs or ISBNs via Alfred Universal Actions or hotkey.

## Installation and requirements

### Requirements
- [Alfred 5](https://www.alfredapp.com/) with Powerpack license.
- macOS 12 (Monterey) or later (Apple Silicon and Intel supported).
- [Pandoc](https://pandoc.org/) (optional, required only for formatted reference pasting).

### Setup
1. Download the latest release (`.alfredworkflow`) from [GitHub Releases](https://github.com/kzssc/alfred-bibtex-citation-picker/releases/latest).
2. Double-click the downloaded file to install it into Alfred.
3. In Alfred Preferences → Workflows → Supercharged Citation Picker, configure your settings:
    - **BibTeX library path**: Absolute path to your `.bib` file (required).
    - **Folders** (optional): Paths to your PDF storage folder, literature notes directory, and CSL style folder.
    - **Open entries in**: Choose Zotero, BibDesk, VS Code, Sublime Text, or default application.
    - **BookOrbit** (optional): Server address, username, and password.

## Basic usage
Trigger the citation picker by typing the keyword `ct` in Alfred, or assign a global hotkey in the workflow settings.

### Search syntax
- **General search**: Type any combination of words from the title, author surname, publication year, or journal.
- **Citekey search**: Prefix a term with `@` (e.g. `@smith2024`).
- **Tag / Keyword search**: Prefix a term with `#` (e.g. `#ethics`).
- **Literature note filter**: Include `*` in the query to display only entries that have an existing literature note.
- **PDF filter**: Include `pdf` in the query to display only entries that have an associated PDF.
- **Combined queries**: Search terms can be freely combined (e.g. `2024 #ai @smith`).

### Citation actions
- <kbd>⏎</kbd>: Paste the citekey formatted in the active citation style.
- <kbd>⌥</kbd><kbd>⏎</kbd>: Add another citation to chain multiple references.
- <kbd>⌘</kbd><kbd>⏎</kbd>: Prompt for a page number before pasting. Confirm with <kbd>⏎</kbd> to paste, or with <kbd>⌥</kbd><kbd>⏎</kbd> to continue adding references.
- <kbd>⇧</kbd><kbd>⌘</kbd><kbd>⏎</kbd>: Paste an inline citation (e.g. `@smith2024` instead of `[@smith2024]`).

### URL and preview actions
- <kbd>⇧</kbd>: Preview the entry attachment (e.g. PDF) in Quick Look. Falls back to web URL or DOI if no local file exists.
- <kbd>⌃</kbd><kbd>⏎</kbd>: Open the entry URL or DOI in your default web browser.
- <kbd>⌘</kbd><kbd>C</kbd>: Copy the entry URL or DOI to the clipboard.
- <kbd>⌘</kbd><kbd>L</kbd>: Display unabridged title, abstract, and keywords in Alfred Large Type view.

### Result icons
| Icon | Description |
| :--- | :--- |
| 🌐 | Entry contains a valid URL or DOI |
| 📄 | Entry includes an abstract |
| 🏷 n | Entry contains *n* keywords / tags |
| 📓 | Literature note exists for this citekey |
| 📕 | Associated PDF exists in library folder |
| 📎 | Entry specifies an attachment or local file |
| 2️⃣ | Entry originated from secondary BibTeX library |
| 📖 | BookOrbit book result |
| 📚 | BookOrbit series result |
| 👤 | BookOrbit author result |

## BookOrbit integration
If you use a [BookOrbit](https://github.com) server, this workflow integrates your book catalog directly into Alfred:
- **Combined search**: When configured, BookOrbit results appear automatically in the citation picker after top BibTeX matches.
- **Dedicated search**: Use the `bo` keyword (configurable) to search BookOrbit catalog items exclusively.
- **BookOrbit actions**:
    - <kbd>⏎</kbd>: Open the selected book, series, or author page in BookOrbit.
    - <kbd>⌃</kbd><kbd>⏎</kbd>: Open the direct web link.
    - <kbd>⌘</kbd><kbd>C</kbd>: Copy the item URL.
- **Credential security**: Passwords are used only once to authenticate and generate tokens. Tokens are stored locally in Alfred's data directory. Credentials are excluded from workflow exports and git tracking.

## Advanced features

### Formatted bibliography pasting
- <kbd>⌘</kbd><kbd>⌥</kbd><kbd>⏎</kbd>: Formats and pastes a complete bibliographic entry for the selected citekey.
- Uses Pandoc with your configured `.csl` style (defaults to APA 7th edition located in `support/apa-7th.csl`).
- Single-entry extraction parses only the selected record, eliminating delay on large libraries.

### Literature notes
- <kbd>⌃</kbd><kbd>⌥</kbd><kbd>⏎</kbd>: Open or create a literature note matching `{citekey}.md`.
- When the note folder is located within an [Obsidian](https://obsidian.md/) vault, notes open directly inside Obsidian.
- If no note exists yet, a new Markdown file named after the citekey is generated immediately.

### PDF and attachment actions
- <kbd>⇧</kbd><kbd>⏎</kbd>: Reveal the reference in your preferred manager or editor (Zotero, BibDesk, VS Code, Sublime Text).
- <kbd>Hyper</kbd><kbd>⏎</kbd>[^1]: Open the PDF matching `{citekey}.pdf` in your default PDF viewer.
- <kbd>⌃</kbd><kbd>⌥</kbd><kbd>⌘</kbd><kbd>⏎</kbd>: Open attachment listed under `file`, `local-url`, or `attachment` BibTeX fields. Zotero storage attachments open automatically through the Zotero app protocol.
- <kbd>fn</kbd><kbd>⏎</kbd>: Auto-file the currently selected PDF in Finder into your library folder, renaming it to `{citekey}_{title}.pdf`.

### Adding entries via DOI or ISBN
- Select any DOI, ISBN, or URL containing a DOI in any macOS application.
- Trigger Alfred Universal Actions or assign a dedicated hotkey to add the metadata directly to your primary `.bib` file.

### Auxiliary commands
Type `scp` in Alfred to access workflow utilities:
- **Change citation format**: Switch active syntax between Pandoc, LaTeX, Multi-Markdown, Org mode, eta, wikilinks, tags, or plain citekey.
- **Citation style search**: Search and install citation styles (`.csl`) from the official repository into your CSL folder.
- **Force buffer reload**: Rebuild cache index immediately if external changes are not yet reflected.
- **Cheat sheet**: Display available keyboard shortcuts and actions.

## Citation formats
| Style | Example output | Page number support |
| :--- | :--- | :--- |
| Pandoc | `[@citekey, p. 42]` | Yes |
| LaTeX | `\autocite[p. 42]{citekey}` | No |
| Multi-Markdown | `[p. 42][#citekey]` | No |
| Org mode | `[cite:@citekey, p. 42]` | Yes |
| eta template | `{% cite citekey %}` | No |
| Obsidian wikilink | `[[citekey\|p. 42]]` | Yes (as alias) |
| iA Writer | `[^citekey, p. 42]` | Yes |
| Tag | `#citekey` | No |
| Bare citekey | `citekey` | No |
| Formatted single entry | *Full APA reference* | No |

## Cite this project
If you reference this software in an academic context, please cite:

```bibtex
@software{Zhang_Supercharged_Citation_Picker_2026,
  author = {Zhang, S. Chris and Grieser, Christopher},
  title = {Supercharged Citation Picker (Optimized)},
  url = {https://github.com/kzssc/alfred-bibtex-citation-picker},
  year = {2026}
}
```

## Credits
- Original workflow concept and architecture by [Chris Grieser](https://github.com/chrisgrieser).
- Icons provided by [Freepik via Flaticon](https://www.flaticon.com/authors/freepik).

[^1]: `Hyper` denotes the macOS hyper modifier key (`⌘⌥⌃⇧`), configurable via tools such as [Karabiner-Elements](https://karabiner-elements.pqrs.org/) or [Hyperkey](https://hyperkey.app/).
