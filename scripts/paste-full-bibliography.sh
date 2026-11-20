#!/bin/zsh
# shellcheck disable=SC2154
export PATH=/usr/local/lib:/usr/local/bin:/opt/homebrew/bin/:$PATH

# GUARD
if ! command -v pandoc &>/dev/null; then
	echo -n "You need to install \`pandoc\` for this feature."
	return 1
fi
#───────────────────────────────────────────────────────────────────────────────

citekey="$*"
csl=$([[ -f "$csl_for_pandoc" ]] && echo "$csl_for_pandoc" || echo "./support/apa-7th.csl")
library="$bibtex_library_path"

temp_bib=$(mktemp "${TMPDIR:-/tmp}/cite_temp.XXXXXX.bib")
if [[ -x "./scripts/write-citation-picker-buffer" ]]; then
	"./scripts/write-citation-picker-buffer" extract-entry "$citekey" > "$temp_bib"
	library="$temp_bib"
else
	rm -f "$temp_bib"
	temp_bib=""
fi

dummydoc="---
nocite: '@$citekey'
---"

reference=$(echo -n "$dummydoc" |
	command pandoc --citeproc --read=markdown --write=plain --wrap=none \
	--csl="$csl" --bibliography="$library" 2>&1)

if [[ -n "$temp_bib" && -f "$temp_bib" ]]; then
	rm -f "$temp_bib"
fi

#───────────────────────────────────────────────────────────────────────────────
# paste via Alfred
echo -n "$reference"
