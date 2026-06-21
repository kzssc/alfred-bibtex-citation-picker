#!/bin/zsh
# shellcheck disable=2154
#───────────────────────────────────────────────────────────────────────────────

buffer="$alfred_workflow_data/buffer.json"
last_version_file="$alfred_workflow_data/last_version.txt"

# create folder and last version file, if not existing yet (e.g. first run)
[[ -d "$alfred_workflow_data" ]] || mkdir -p "$alfred_workflow_data"
[[ -e "$last_version_file" ]] || echo "never run" > "$last_version_file"

# RELOAD BUFFER if buffer is outdated compared to…
# - library or 2nd library file
# - literature notes
# - PDFs
# - workflow preferences (potentially changing matching behavior etc)
# - manually requested reload
# - new workflow version (to ensure new features/bug fixes take effect)
if [[ "$bibtex_library_path" -nt "$buffer" ]] ||
	[[ "$secondary_library_path" -nt "$buffer" ]] ||
	[[ "$literature_note_folder" -nt "$buffer" ]] ||
	[[ "$pdf_folder" -nt "$buffer" ]] ||
	[[ "./prefs.plist" -nt "$buffer" ]] ||
	[[ "$buffer_reload" == "true" ]] ||
	[[ "$(head -n1 "$last_version_file")" != "$alfred_workflow_version" ]] \
	; then
	# Rebuild PDF and Obsidian literature note file lists in parallel background tasks
	pdf_list_file="$alfred_workflow_data/pdf_list.txt"
	lit_list_file="$alfred_workflow_data/lit_list.txt"
	if [[ -d "$pdf_folder" ]]; then
		find "$pdf_folder" -type f -name "*.pdf" | awk -F/ '{print $NF}' | sed -E 's/\.pdf$//; s/_[^_]*$//' > "$pdf_list_file" &
	else
		echo -n "" > "$pdf_list_file" &
	fi
	if [[ -d "$literature_note_folder" ]]; then
		find "$literature_note_folder" -type f -name "*.md" | awk -F/ '{print $NF}' | sed 's/\.md$//' > "$lit_list_file" &
	else
		echo -n "" > "$lit_list_file" &
	fi
	wait

	osascript -l JavaScript "./scripts/write-citation-picker-buffer.js" > "$buffer"
	echo -n "$alfred_workflow_version" > "$last_version_file"
fi

# pass json to Alfred
cat "$buffer"
