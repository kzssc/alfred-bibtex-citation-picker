# Supercharged citation picker

## About this fork
This is a fork of [chrisgrieser/alfred-bibtex-citation-picker](https://github.com/chrisgrieser/alfred-bibtex-citation-picker) with the following changes on top of upstream:
- **Go-powered performance rewrite:** Cache loading, single-citation pasting,
  and attachment/literature-note opening now run through a compiled Go binary
  (`src/rebuild-cache/main.go`) instead of JS/shell scripts. Single-citation
  pasting is about 22x faster, and attachment/note opening is near-instant.
- **Zotero PDF opening:** PDFs stored inside Zotero (not just local files) can
  now be opened directly via the Zotero URL scheme.
