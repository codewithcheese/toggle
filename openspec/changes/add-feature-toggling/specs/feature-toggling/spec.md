# Spec Delta

## Purpose

Install optional repo setup only while needed, protecting edits through explicit source preservation and providing inspectable conflicts, previews, and status.

## ADDED Requirements

### Requirement: Turn a named feature on and off

`toggle on <name>` SHALL install the selected feature's source files. `toggle off <name>` SHALL remove its recorded installed files while preserving unrelated files. Off SHALL use recorded destinations rather than selecting removal targets from the current source tree. Normal cleanup SHALL require an available global or repo-local definition of the feature.

#### Scenario: Remove installed files and preserve unrelated files
- **GIVEN** review has installed `.agents/skills/review/SKILL.md`
- **AND** an unrelated `.agents/skills/personal/SKILL.md` exists
- **WHEN** `toggle off review` runs from the repo root
- **THEN** the installed review skill is removed and review is off
- **AND** the personal skill and feature sources remain unchanged

#### Scenario: Ignore new source paths during off
- **GIVEN** review has installed `old.md` unchanged
- **AND** its source has since replaced `old.md` with `new.md`
- **AND** an unrelated repo `new.md` exists
- **WHEN** `toggle off review` runs from the repo root
- **THEN** `old.md` is removed
- **AND** repo `new.md` and the current source tree remain unchanged

#### Scenario: Report a removed definition
- **GIVEN** review is installed but no global or repo-local definition for review remains
- **WHEN** `toggle off review` runs from the repo root
- **THEN** the command fails and reports the missing definition and need for manual cleanup
- **AND** installed files, exclusions, and records remain unchanged

#### Scenario: Clean up after removing a local override
- **GIVEN** review was installed from `.toggle/review/source/instructions.md`
- **AND** repo `instructions.md` is an untracked ordinary file matching its at-on content and executable mode at its recorded destination
- **AND** the local review definition has been removed but global `review/source/` remains available
- **WHEN** `toggle off review` runs from the repo root
- **THEN** it succeeds and removes the installed file, review's exclusions, and its installation entry
- **AND** the global definition remains unchanged

### Requirement: Run on and off at the repo root

On and off, including dry runs, SHALL require the working directory to be the Git repo root. Calls from subdirectories or outside Git SHALL fail before changes, with a diagnostic explaining the root requirement.

#### Scenario: Reject on from a subdirectory
- **GIVEN** review is off and the working directory is a Git repo subdirectory
- **WHEN** `toggle on review` runs
- **THEN** it fails and tells the user to run from the repo root
- **AND** files, exclusions, and records remain unchanged

#### Scenario: Reject off from a subdirectory
- **GIVEN** review is installed and the working directory is a Git repo subdirectory
- **WHEN** `toggle off review` runs
- **THEN** it fails and tells the user to run from the repo root
- **AND** files, exclusions, and records remain unchanged

#### Scenario: Reject an on preview from a subdirectory
- **GIVEN** the working directory is a Git repo subdirectory
- **WHEN** `toggle on review --dry-run` runs
- **THEN** it fails with the repo-root requirement and creates no files

#### Scenario: Reject an off preview from a subdirectory
- **GIVEN** the working directory is a Git repo subdirectory
- **WHEN** `toggle off review --dry-run` runs
- **THEN** it fails with the repo-root requirement and changes no files

#### Scenario: Reject outside Git
- **GIVEN** the working directory is outside Git
- **WHEN** `toggle on review` runs
- **THEN** it fails with the Git repo-root requirement and creates no files

### Requirement: Make repeated commands harmless

On for an already installed feature SHALL succeed without reinstalling files, switching definitions, or overwriting edits. Off for an available feature already off SHALL succeed without changes. Repeated commands SHALL NOT refresh source changes automatically.

#### Scenario: Preserve edits and source choice when on is repeated
- **GIVEN** review is installed from its global source and its installed file has an edit
- **AND** a repo-local review definition has since been added
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it succeeds and reports review is already on
- **AND** installed content, exclusions, and installation bookkeeping remain unchanged

#### Scenario: Leave an off feature unchanged
- **GIVEN** review is available and already off
- **WHEN** `toggle off review` runs from the repo root
- **THEN** it succeeds without changing files, exclusions, or records

### Requirement: Reuse matching untracked files

On SHALL silently accept a pre-existing untracked file matching source content and executable mode if no other installed feature owns its resolved destination. It SHALL record that file as installed so normal off removes it. Tracked files and files owned by another installed feature SHALL conflict even when matching.

#### Scenario: Accept a matching untracked file
- **GIVEN** review is off and its destination is untracked, matches the source, and is not installed for another feature
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it succeeds without prompting and records the file as installed for review
- **AND** the file remains matching and Git reports it as ignored

#### Scenario: Remove an accepted file during off
- **GIVEN** review was turned on by accepting a matching pre-existing untracked file
- **AND** that file remains unchanged
- **WHEN** `toggle off review` runs from the repo root
- **THEN** the file is removed and review is off

#### Scenario: Refuse a matching file owned by another feature
- **GIVEN** checks has installed `instructions.md`
- **AND** review's `source/instructions.md` matches it
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and identifies both features and the shared path
- **AND** installed files, exclusions, and records remain unchanged

### Requirement: Require explicit preservation of edits

Off SHALL remove an installed file only if it is unchanged in content and executable mode since on, already absent, or matches the same relative path in the currently selected source in content and executable mode. Other edits SHALL conflict. Toggle SHALL NOT save edited copies or update feature sources automatically.

#### Scenario: Block an unpreserved edit
- **GIVEN** review is installed and its skill content or executable mode has been edited
- **AND** the selected source does not match that edit
- **WHEN** `toggle off review` runs from the repo root
- **THEN** it fails and reports the edited file and that the edit is not preserved in the selected source
- **AND** files, sources, exclusions, and records remain unchanged

#### Scenario: Remove edits explicitly saved globally
- **GIVEN** review is installed and its skill has been edited
- **AND** the user has updated the global source to match its content and executable mode
- **AND** there is no repo-local override
- **WHEN** `toggle off review` runs from the repo root
- **THEN** it succeeds and removes the installed skill
- **AND** the updated global source remains unchanged

#### Scenario: Preserve edits as a repo-local feature
- **GIVEN** review was installed globally and its skill has been edited
- **AND** the user has created `.toggle/review/source/` containing matching content and executable mode at the same relative path
- **WHEN** `toggle off review` runs from the repo root
- **THEN** it succeeds and removes the installed skill
- **AND** the local definition preserves the edit and the global definition remains unchanged

#### Scenario: Reinstall from the local source
- **GIVEN** review is off and `.toggle/review/source/` holds the user's saved skill edit
- **AND** the destination is absent
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it installs the local source content and executable mode
- **AND** neither source definition is changed

#### Scenario: Remove an unchanged file after global source changes
- **GIVEN** review's installed file still matches its content and executable mode recorded at on
- **AND** its global source has since changed
- **WHEN** `toggle off review` runs from the repo root
- **THEN** it succeeds and removes the installed file
- **AND** the changed source remains unchanged

### Requirement: Check conflicts before changes

Toggle SHALL validate all planned file, exclusion, and bookkeeping changes before applying any. Differing pre-existing files, incompatible file types, duplicate resolved destinations, and targets owned by other installed features SHALL conflict without overwriting files or applying exclusions.

#### Scenario: Stop the whole operation on a conflicting file
- **GIVEN** review contains one absent target and another target containing different untracked content
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and identifies the differing target
- **AND** the absent target remains absent and files, exclusions, and records remain unchanged

#### Scenario: Reject two source paths resolving to one destination
- **GIVEN** two files in review's source tree map through repo symlinks to the same resolved destination
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and reports the overlapping paths
- **AND** files, symlinks, exclusions, and records remain unchanged

### Requirement: Report conflicts by feature and path

Conflicts SHALL return nonzero and identify the feature, relevant paths, and a short reason. Toggle SHALL NOT generate content diffs; users and agents can compare the reported paths using their own tools.

#### Scenario: Identify an existing-content conflict
- **GIVEN** repo `instructions.md` differs from review's `source/instructions.md`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it fails and reports review, both file paths, and the existing-content conflict
- **AND** it prints no content diff and does not change either file

### Requirement: Report feature status for the current repo

`toggle status` SHALL report available features and recorded installations for the current repo as `off`, `on`, `modified`, or `conflicted`. It SHALL identify relevant paths and reasons, including which next transition is blocked. Status SHALL NOT change files, exclusions, or records.

#### Scenario: Report state for the current repo
- **GIVEN** global review contains only `source/instructions.md` and is installed in repo A
- **AND** repo B has no installed features, no ignore rules, and no tracked or existing `instructions.md`
- **WHEN** `toggle status` runs from repo B's root
- **THEN** it reports review as off
- **AND** neither repo is changed

### Requirement: Inspect status from anywhere inside the repo

`toggle status` SHALL inspect the containing Git repo from its root or any subdirectory. Outside Git it SHALL return nonzero with a diagnostic that a Git repo is required. These calls SHALL NOT change files, exclusions, or records.

#### Scenario: Inspect status from a repo subdirectory
- **GIVEN** review is installed from its available global source
- **AND** its only installed file is an untracked ordinary `instructions.md` matching its at-on content and executable mode at its recorded destination
- **AND** the working directory is the repo's `docs/` subdirectory
- **WHEN** `toggle status` runs
- **THEN** it succeeds and reports review as on for the containing repo
- **AND** files, exclusions, and records remain unchanged

#### Scenario: Reject status outside Git
- **GIVEN** review is globally available and the working directory is outside Git
- **WHEN** `toggle status` runs
- **THEN** it returns nonzero and explains that a Git repo is required
- **AND** no files or directories are created or changed

### Requirement: Classify status with conflicts first

Status SHALL validate off for installed features and on otherwise. A validation failure SHALL yield `conflicted`; otherwise missing installed files or content/executable-mode differences from the at-on baseline SHALL yield `modified`; otherwise installed features SHALL be `on` and uninstalled features `off`. Source changes alone SHALL NOT change the installed baseline.

#### Scenario: Report an unchanged installation
- **GIVEN** review is installed from its available global source
- **AND** its only installed file is an untracked ordinary `instructions.md` matching its at-on content and executable mode at its recorded destination
- **WHEN** `toggle status` runs from the repo root
- **THEN** it reports review as on

#### Scenario: Report an unpreserved edit as conflicted
- **GIVEN** review's installed skill has an edit that does not match the selected source
- **WHEN** `toggle status` runs from the repo root
- **THEN** it reports review as conflicted and identifies the edit as blocking off

#### Scenario: Report a source-preserved edit as modified
- **GIVEN** review is installed and its untracked ordinary `.agents/skills/review/SKILL.md` has edited content at its recorded destination, with no destination symlinks
- **AND** `.toggle/review/source/.agents/skills/review/SKILL.md` matches the installed content and executable mode
- **WHEN** `toggle status` runs from the repo root
- **THEN** it reports review as modified and identifies the skill path

#### Scenario: Prefer a tracking conflict over a preserved edit
- **GIVEN** review's installed skill differs from its at-on baseline but matches the selected source
- **AND** the skill has been force-added to Git's index
- **WHEN** `toggle status` runs from the repo root
- **THEN** it reports review as conflicted and identifies the tracked skill as blocking off

#### Scenario: Report a missing file as modified
- **GIVEN** review was installed from its still-available global source containing only `source/instructions.md`
- **AND** repo `instructions.md` has been deleted, remains untracked, and its declared path still resolves to the recorded destination
- **WHEN** `toggle status` runs from the repo root
- **THEN** it reports review as modified and identifies the absent file

#### Scenario: Report a missing definition as conflicted
- **GIVEN** review is installed but its global and local definitions are absent
- **WHEN** `toggle status` runs from the repo root
- **THEN** it reports review as conflicted and explains that off requires manual cleanup because the definition is missing

#### Scenario: Report source drift without changing the baseline
- **GIVEN** review's only installed file is an untracked ordinary `instructions.md` matching its at-on content and executable mode at its recorded destination
- **AND** the selected global `review/source/instructions.md` has changed since on
- **WHEN** `toggle status` runs from the repo root
- **THEN** it reports review as on

### Requirement: Preview on and off without changes

On and off SHALL support `--dry-run` with the same validation as actual operations. Successful previews SHALL show planned file and exclusion changes. Previews SHALL NOT change files, sources, exclusions, or records.

#### Scenario: Preview on
- **GIVEN** review is off and its destinations are absent and untracked
- **WHEN** `toggle on review --dry-run` runs from the repo root
- **THEN** it lists the proposed installations and exact local exclusions
- **AND** review remains off and no files, exclusions, or records change

#### Scenario: Preview off with preserved edits
- **GIVEN** review is installed and its edited skill matches the selected local source
- **WHEN** `toggle off review --dry-run` runs from the repo root
- **THEN** it lists the proposed removals and exclusion changes
- **AND** installed files, sources, exclusions, and records remain unchanged

#### Scenario: Report an unpreserved edit during preview
- **GIVEN** review is installed with an edited skill that does not match the selected source
- **WHEN** `toggle off review --dry-run` runs from the repo root
- **THEN** it fails with the same edited-file conflict as off
- **AND** files, exclusions, and records remain unchanged

### Requirement: Report partial application after write failures

If an on/off write fails, toggle SHALL stop further writes and return nonzero, reporting the feature, completed effects, failed path, remaining effects, and bookkeeping and exclusion locations for inspection. It SHALL leave completed effects in place for manual cleanup without automatic rollback. The failed write itself can have partial effects.

#### Scenario: Stop on after a file write fails
- **GIVEN** global review contains only `source/first.md`, `source/second.md`, and `source/third.md`
- **AND** the repo has no installed features, ignore rules, or tracked or existing target files
- **AND** writing `second.md` will fail after `first.md` has been written, while `third.md` is still pending
- **WHEN** `toggle on review` runs from the repo root
- **THEN** it returns nonzero, leaves `first.md` installed, and does not attempt to write `third.md`
- **AND** it reports review, the completed effects, failed `second.md` write, remaining effects, and bookkeeping and exclusion locations
- **AND** it does not roll back completed effects or claim the failed write left `second.md` untouched

#### Scenario: Stop off after a removal fails
- **GIVEN** review is installed from its available global definition
- **AND** its only installed files are untracked ordinary `first.md`, `second.md`, and `third.md`, matching their at-on content and executable modes at their recorded destinations
- **AND** removing `second.md` will fail after `first.md` has been removed, while `third.md` is still pending
- **WHEN** `toggle off review` runs from the repo root
- **THEN** it returns nonzero, leaves `first.md` removed, and does not attempt to remove `third.md`
- **AND** it reports review, the completed effects, failed `second.md` removal, remaining effects, and bookkeeping and exclusion locations
- **AND** it does not roll back completed effects or continue with exclusion or bookkeeping removal
