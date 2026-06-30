#!/bin/zsh
# shellcheck disable=2154
#───────────────────────────────────────────────────────────────────────────────

buffer="$alfred_workflow_data/buffer.json"
last_version_file="$alfred_workflow_data/last_version.txt"
rebuild_pid_file="$alfred_workflow_data/rebuild.pid"

# create folder and last version file, if not existing yet (e.g. first run)
[[ -d "$alfred_workflow_data" ]] || mkdir -p "$alfred_workflow_data"
[[ -e "$last_version_file" ]] || echo "never run" > "$last_version_file"

rebuild_buffer() {
	pdf_list_file="$alfred_workflow_data/pdf_list.txt"
	lit_list_file="$alfred_workflow_data/lit_list.txt"

	local rebuild_pdf=false
	if [[ -d "$pdf_folder" ]]; then
		if [[ ! -f "$pdf_list_file" ]] || [[ "$pdf_folder" -nt "$pdf_list_file" ]]; then
			rebuild_pdf=true
		fi
	else
		echo -n "" > "$pdf_list_file"
	fi

	local rebuild_lit=false
	if [[ -d "$literature_note_folder" ]]; then
		if [[ ! -f "$lit_list_file" ]] || [[ "$literature_note_folder" -nt "$lit_list_file" ]]; then
			rebuild_lit=true
		fi
	else
		echo -n "" > "$lit_list_file"
	fi

	if [[ "$rebuild_pdf" == "true" ]] && [[ "$rebuild_lit" == "true" ]]; then
		find "$pdf_folder" -maxdepth 2 -type f -name "*.pdf" | awk -F/ '{print $NF}' | sed -E 's/\.pdf$//; s/_[^_]*$//' > "$pdf_list_file" &
		find "$literature_note_folder" -type f -name "*.md" | awk -F/ '{print $NF}' | sed 's/\.md$//' > "$lit_list_file" &
		wait
	elif [[ "$rebuild_pdf" == "true" ]]; then
		find "$pdf_folder" -maxdepth 2 -type f -name "*.pdf" | awk -F/ '{print $NF}' | sed -E 's/\.pdf$//; s/_[^_]*$//' > "$pdf_list_file"
	elif [[ "$rebuild_lit" == "true" ]]; then
		find "$literature_note_folder" -type f -name "*.md" | awk -F/ '{print $NF}' | sed 's/\.md$//' > "$lit_list_file"
	fi

	if [[ -x "./scripts/write-citation-picker-buffer" ]]; then
		"./scripts/write-citation-picker-buffer" > "$buffer"
	else
		# Find node executable
		local node_bin=""
		if [[ -x "/opt/homebrew/bin/node" ]]; then
			node_bin="/opt/homebrew/bin/node"
		elif [[ -x "/usr/local/bin/node" ]]; then
			node_bin="/usr/local/bin/node"
		elif command -v node >/dev/null 2>&1; then
			node_bin="node"
		fi

		if [[ -n "$node_bin" ]]; then
			"$node_bin" "./scripts/write-citation-picker-buffer.js" > "$buffer"
		else
			osascript -l JavaScript "./scripts/write-citation-picker-buffer.js" > "$buffer"
		fi
	fi
	echo -n "$alfred_workflow_version" > "$last_version_file"
	rm -f "$rebuild_pid_file"
}

# RELOAD BUFFER if buffer is outdated compared to…
# - library or 2nd library file
# - literature notes
# - PDFs
# - workflow preferences (potentially changing matching behavior etc)
# - manually requested reload
# - new workflow version (to ensure new features/bug fixes take effect)
needs_rebuild=false
if [[ ! -f "$buffer" ]] ||
	[[ "$bibtex_library_path" -nt "$buffer" ]] ||
	[[ "$secondary_library_path" -nt "$buffer" ]] ||
	[[ "$literature_note_folder" -nt "$buffer" ]] ||
	[[ "$pdf_folder" -nt "$buffer" ]] ||
	[[ "./prefs.plist" -nt "$buffer" ]] ||
	[[ "$buffer_reload" == "true" ]] ||
	[[ "$(head -n1 "$last_version_file")" != "$alfred_workflow_version" ]] \
	; then
	needs_rebuild=true
fi

if [[ "$needs_rebuild" == "true" ]]; then
	if [[ -f "$buffer" ]]; then
		# Print the cached version immediately to ensure instant loading speed
		cat "$buffer"

		# Rebuild in the background if not already running
		should_spawn=true
		if [[ -f "$rebuild_pid_file" ]]; then
			old_pid=$(cat "$rebuild_pid_file" 2>/dev/null)
			if [[ -n "$old_pid" ]] && kill -0 "$old_pid" 2>/dev/null; then
				should_spawn=false
			fi
		fi

		if [[ "$should_spawn" == "true" ]]; then
			(
				echo $$ > "$rebuild_pid_file"
				rebuild_buffer
			) >/dev/null 2>&1 &!
		fi
	else
		# No cached buffer exists (e.g. first run). Must run synchronously.
		rebuild_buffer
		cat "$buffer"
	fi
else
	# Buffer is up to date, output it directly
	cat "$buffer"
fi
