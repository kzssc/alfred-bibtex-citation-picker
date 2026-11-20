set quiet := true

# REQUIRED local workflow uses same folder name

workflow_uid := `basename "$PWD"`
prefs_location := `defaults read com.runningwithcrayons.Alfred-Preferences syncfolder | sed "s|^~|$HOME|"`
local_workflow := prefs_location / "Alfred.alfredpreferences/workflows" / workflow_uid

#───────────────────────────────────────────────────────────────────────────────

transfer-changes-FROM-local:
    #!/usr/bin/env zsh
    rsync --archive --delete --exclude-from="$PWD/.rsync-exclude" "{{ local_workflow }}/" "$PWD"
    git status --short

transfer-changes-TO-local:
    #!/usr/bin/env zsh
    rsync --archive --delete --exclude-from="$PWD/.rsync-exclude" "$PWD/" "{{ local_workflow }}"
    cd "{{ local_workflow }}"
    print "\e[1;34mChanges at the local workflow:\e[0m"
    git status --short .

[macos]
open-local-workflow-in-alfred:
    #!/usr/bin/env zsh
    # using JXA and URI for redundancy, as both are not 100 % reliable https://www.alfredforum.com/topic/18390-get-currently-edited-workflow-uri/
    open "alfredpreferences://navigateto/workflows>workflow>{{ workflow_uid }}"
    osascript -e 'tell application id "com.runningwithcrayons.Alfred" to reveal workflow "{{ workflow_uid }}"'

build-binary:
    #!/usr/bin/env zsh
    cd src/rebuild-cache
    GOOS=darwin GOARCH=arm64 go build -trimpath -o /tmp/bin_arm64 .
    GOOS=darwin GOARCH=amd64 go build -trimpath -o /tmp/bin_amd64 .
    lipo -create -output ../../scripts/write-citation-picker-buffer /tmp/bin_arm64 /tmp/bin_amd64
    rm -f /tmp/bin_arm64 /tmp/bin_amd64
    chmod +x ../../scripts/write-citation-picker-buffer

release:
    ./.build-and-release.sh
