#!/bin/zsh

if [[ -x "./scripts/write-citation-picker-buffer" ]]; then
	"./scripts/write-citation-picker-buffer" open-litnote "$@"
else
	osascript -l JavaScript "./scripts/open-create-literature-note.js" "$@"
fi
