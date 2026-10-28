# Spec Delta

## Purpose

Keep installed optional setup out of ordinary Git staging while protecting tracked files, repo-local definitions, and existing ignore rules.

## ADDED Requirements

### Requirement: Check tracked files before changes or exclusions

Before changing a target or adding its exclusion, toggle SHALL verify that Git tracks neither its declared path nor its resolved destination. Matching content and existing ignore rules SHALL NOT bypass this check. A tracked target SHALL conflict before any planned changes.

#### Scenario: Refuse a matching committed target
- **GIVEN** review targets a file matching its source that is committed in the repo
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and reports the tracked target
- **AND** repo files, exclusions, and records remain unchanged

#### Scenario: Refuse off after an installed file is staged
- **GIVEN** review is installed and its skill has been added to Git's index with `git add -f`
- **WHEN** `toggle off review` runs from the repo root
- **THEN** it fails and reports the tracked skill
- **AND** the file, index, exclusions, and records remain unchanged

#### Scenario: Refuse a tracked alias to an untracked file
- **GIVEN** review's destination is a committed file symlink resolving to a matching untracked file inside the repo
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and identifies the tracked declared path
- **AND** the symlink, resolved file, index, exclusions, and records remain unchanged

### Requirement: Add exact local exclusions for installed files

On SHALL add exact repo-relative exclusions for resolved files installed or accepted for the feature to Git's local exclude file. Installed files SHALL be effectively ignored. Toggle SHALL NOT hide shared directories or edit versioned ignore files; an overriding rule SHALL conflict before changes.

#### Scenario: Ignore only the installed skill
- **GIVEN** review targets the absent `.agents/skills/review/SKILL.md`
- **AND** unrelated `.agents/skills/personal/SKILL.md` exists and is not ignored
- **WHEN** `toggle on review` runs from the repo root
- **THEN** Git reports the review skill as ignored through the local exclude file
- **AND** the personal skill remains unignored and versioned ignore files remain unchanged

#### Scenario: Keep installed files out of normal staging
- **GIVEN** a global review feature is installed in an otherwise clean repo with effective local exclusions
- **WHEN** `git add .` runs from the repo root
- **THEN** no installed feature files or toggle records are staged
- **AND** `git status --porcelain` reports a clean repo

#### Scenario: Report an overriding ignore rule
- **GIVEN** review targets `.agents/skills/review/SKILL.md`
- **AND** repo `.gitignore` contains `!/.agents/skills/review/SKILL.md`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and identifies the target and overriding rule
- **AND** files, exclusions, and records remain unchanged

### Requirement: Reserve Git control paths

Toggle SHALL reject declared or resolved installation targets named `.gitignore`, and targets that are `.git` entries or lie beneath them, at any depth, before changing files, exclusions, or bookkeeping. This includes symlinks and filesystem case aliases, regardless of Git tracking. Top-level feature metadata outside `source/` SHALL remain outside installation.

#### Scenario: Reject an incoming ignore file before applying any changes
- **GIVEN** review contains `source/.gitignore` with `!/instructions.md` and `source/instructions.md`
- **AND** neither destination exists or is tracked
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and identifies the reserved `.gitignore` target
- **AND** files, exclusions, and bookkeeping remain unchanged

#### Scenario: Reject a nested ignore file during preview
- **GIVEN** review contains `source/config/.gitignore`
- **WHEN** `toggle on review --dry-run` runs from the repo root
- **THEN** it fails and identifies the reserved nested `.gitignore` target
- **AND** no files, exclusions, or bookkeeping change

#### Scenario: Reject a nested Git directory
- **GIVEN** review contains `source/vendor/.git/config`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and identifies the reserved `.git` path
- **AND** files, exclusions, and bookkeeping remain unchanged

#### Scenario: Reject a nested Git pointer file
- **GIVEN** review contains an ordinary `source/vendor/.git` file
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and identifies the reserved `.git` target
- **AND** files, exclusions, and bookkeeping remain unchanged

#### Scenario: Reject a resolved ignore-file alias
- **GIVEN** review contains `source/settings.ignore` matching an untracked repo `config/.gitignore`
- **AND** repo `settings.ignore` is a symlink to `config/.gitignore`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and identifies the alias and reserved resolved `.gitignore` target
- **AND** the symlink, target, exclusions, and bookkeeping remain unchanged

#### Scenario: Reject an alias into a nested Git directory
- **GIVEN** review contains `source/settings/config`
- **AND** repo `settings` is a symlink to `vendor/.git`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and identifies the reserved resolved `.git` path
- **AND** files, symlinks, exclusions, and bookkeeping remain unchanged

### Requirement: Preserve existing exclusions

Toggle SHALL preserve user-written exclusions and entries for other features. Off SHALL remove only its own exclusions after its installed files have been safely removed.

#### Scenario: Preserve other rules across on and off
- **GIVEN** review is off and Git's local exclude file contains user rules and another feature's entries
- **WHEN** `toggle on review` followed by `toggle off review` runs from the repo root
- **THEN** the original rules and other-feature entries remain unchanged
- **AND** review's entries have been removed

### Requirement: Keep installation bookkeeping in Git metadata

Toggle SHALL keep installation bookkeeping in the repo's Git-local metadata, including when `.git` is a file pointing to a separate Git directory. Records SHALL contain installation information without edited-file copies and SHALL remain outside project files and normal staging.

#### Scenario: Store installation information outside project files
- **GIVEN** review is available globally and its destinations are absent
- **WHEN** `toggle on review` runs from the repo root
- **THEN** its installation bookkeeping is stored in the repo's Git-local metadata
- **AND** no toggle record or edited-file cache is created among project files

#### Scenario: Keep bookkeeping and exclusions in a separate Git directory
- **GIVEN** repo `.git` is a file pointing to a separate Git directory
- **AND** global review contains only `source/instructions.md`, with no repo destination or tracked entry
- **AND** the repo has no installed features or ignore rules
- **WHEN** `toggle on review` runs from the repo root
- **THEN** its installation bookkeeping is stored in that separate Git directory
- **AND** its exact exclusion is added to that Git directory's local exclude file and installed `instructions.md` is ignored
- **AND** the `.git` file is unchanged and no toggle record is created among project files

### Requirement: Leave repo-local definitions under owner control

Toggle SHALL NOT add ignore rules for `.toggle/`, modify its definitions, or change its tracked state. Repo-local definitions can be committed or ignored by their owner using ordinary Git configuration, independently of the exclusions for installed copies.

#### Scenario: Use a committed local definition
- **GIVEN** `.toggle/review/source/.agents/skills/review/SKILL.md` is committed
- **AND** its repo destination is absent and untracked
- **WHEN** `toggle on review` followed by `toggle off review` runs from the repo root
- **THEN** the committed definition and Git index remain unchanged
- **AND** no ignore rule for `.toggle/` is added

#### Scenario: Leave untracked definitions visible
- **GIVEN** `.toggle/review/source/instructions.md` is untracked and unignored
- **AND** the destination `instructions.md` is absent
- **WHEN** `toggle on review` runs from the repo root
- **THEN** installed `instructions.md` is ignored
- **AND** the repo-local definition remains untracked and unignored
