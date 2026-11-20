#!/bin/zsh

# goto git root
cd "$(git rev-parse --show-toplevel)" || return 1

#───────────────────────────────────────────────────────────────────────────────

# Prompt for next version number
current_version=$(plutil -extract version xml1 -o - info.plist | sed -n 's/.*<string>\(.*\)<\/string>.*/\1/p')
echo "current version: $current_version"
echo -n "   next version: "
read -r next_version
echo "────────────────────────"

# GUARD
if [[ -z "$next_version" || "$next_version" == "$current_version" ]]; then
	print "\e[1;31mInvalid version number.\e[0m"
	return 1
fi

#───────────────────────────────────────────────────────────────────────────────
# update version number in THE REPO's `info.plist`
plutil -replace version -string "$next_version" info.plist

# update version number in LOCAL `info.plist` (if repo is outside Alfred preferences)
prefs_location=$(defaults read com.runningwithcrayons.Alfred-Preferences syncfolder 2>/dev/null | sed "s|^~|$HOME|")
workflow_uid="$(basename "$PWD")"
if [[ -n "$prefs_location" ]]; then
	local_info_plist="$prefs_location/Alfred.alfredpreferences/workflows/$workflow_uid/info.plist"
	if [[ -f "$local_info_plist" && "$local_info_plist" != "$PWD/info.plist" ]]; then
		plutil -replace version -string "$next_version" "$local_info_plist"
	fi
fi

#───────────────────────────────────────────────────────────────────────────────

# copy download link for current version
msg="Download the latest release here:"
repo=$(git remote --verbose | head -n1 | sed -E 's/.*github\.com[:/]([^[:space:]]+).*/\1/; s/\.git$//')
repo_name="${repo##*/}"
asset_name="$repo_name.alfredworkflow"
url="https://github.com/$repo/releases/download/$next_version/$asset_name"
echo -n "$msg $url" | pbcopy

#───────────────────────────────────────────────────────────────────────────────

# build release package
zip --recurse-paths --symlinks "$asset_name" . \
  --exclude "README.md" "RELEASE_CHECKLIST.md" ".git*" "Justfile" ".build-and-release.sh" \
  ".rsync-exclude" ".editorconfig" ".typos.toml" ".markdownlint.*" \
  "prefs.plist" "*.DS_Store" "src/*" ".rumdl*" ".harper*" \
  "CITATION.cff" "tokens.json" "*.idx"

#───────────────────────────────────────────────────────────────────────────────

# commit, tag, and publish to repository
git add --all &&
	git commit -m "release: $next_version" &&
	git pull --no-progress &&
	git push --no-progress &&
	git tag "$next_version" &&
	git push --no-progress origin --tags &&
	gh release create "$next_version" "$asset_name" --title "$next_version" --generate-notes &&
	rm -f "$asset_name"
