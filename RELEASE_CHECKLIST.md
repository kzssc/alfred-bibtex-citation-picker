---
title: Release Checklist
subtitle: Security and Quality Assurance for Public Releases
author: S. Chris Zhang^[Law, Universitat Pompeu Fabra, Barcelona / academic@czhang.uk ]
lang: en-gb
abstract: Operational checklist for releasing the Supercharged Citation Picker Alfred workflow publicly without leaking credentials, personal paths, or development artifacts.
note:
---

# Release checklist

This checklist outlines the verification steps required before releasing a new version of the Supercharged Citation Picker workflow. Complete each step to ensure that public releases contain no sensitive credentials, personal paths, or broken binaries.

## 1. Secrets and privacy verification

Configuration files and environment defaults must remain free of personal credentials.

- [ ] **Check `info.plist` export protection**:
  Ensure that `<key>variablesdontexport</key>` includes all user-specific credentials:
  - `BOOKORBIT_PASSWORD`
  - `BOOKORBIT_URL`
  - `BOOKORBIT_USERNAME`
- [ ] **Check `info.plist` default values**:
  Verify that all `<key>default</key>` entries in `userconfigurationconfig` contain empty strings or generic placeholders (`https://bookorbit.example.com`). No local paths (e.g. `/Users/...`) may exist in default values.
- [ ] **Verify `prefs.plist` isolation**:
  Confirm that `prefs.plist` remains uncommitted and untracked:
  ```bash
  git status --ignored | grep prefs.plist
  ```
- [ ] **Verify session token and cache isolation**:
  Ensure that no `.data/` directory, `tokens.json` file, or `*.idx` files are tracked in git:
  ```bash
  git status
  ```

## 2. Binary compilation and code integrity

The compiled binary must contain universal architecture support and clean symbol paths.

- [ ] **Rebuild the universal binary with path trimming**:
  Compile both `arm64` and `amd64` slices using `-trimpath` to eliminate local developer paths from symbol tables:
  ```bash
  just build-binary
  ```
- [ ] **Verify universal binary architectures**:
  Confirm that the executable contains both architectures:
  ```bash
  lipo -info scripts/write-citation-picker-buffer
  # Output must state: Architectures in the fat file: ... are: x86_64 arm64
  ```
- [ ] **Check binary strings for leaked paths**:
  Ensure that no personal home directory paths remain embedded inside the binary:
  ```bash
  strings scripts/write-citation-picker-buffer | grep "/Users/"
  # Only expected code patterns (e.g. ";/Users/") should match.
  ```
- [ ] **Verify execution permissions**:
  Ensure the binary is executable:
  ```bash
  chmod +x scripts/write-citation-picker-buffer
  ```

## 3. Metadata and documentation consistency

Version numbers and documentation must match across the repository.

- [ ] **Update `CITATION.cff`**:
  Verify that `version` and `date-released` match the target release version and current date.
- [ ] **Update `README.md`**:
  Document any new features, configuration flags, or keybindings introduced in this version.
- [ ] **Check git status**:
  Ensure all intended source files in `src/rebuild-cache/` and icons in `icons/` are staged:
  ```bash
  git status --short
  ```

## 4. Packaging and release execution

Automated release scripts package the workflow and publish directly to the repository.

- [ ] **Execute the release script**:
  Run the automated release command:
  ```bash
  just release
  ```
  Provide the target semantic version string when prompted.
- [ ] **Verify GitHub release page**:
  Open the release page on GitHub and verify:
  - The tag matches the version.
  - The workflow package (`alfred-bibtex-citation-picker.alfredworkflow`) is attached.
  - Release notes reflect the changes.
- [ ] **Verify clean git state**:
  Ensure the working tree is clean and synced with remote:
  ```bash
  git status
  ```
