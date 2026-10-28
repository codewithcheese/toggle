# Spec Delta

## Purpose

Make reusable skills, agents, and files discoverable from global and repo-local feature directories, with predictable names and destination paths.

## ADDED Requirements

### Requirement: Discover features by directory name

Toggle SHALL discover immediate directories containing `source/` under `~/.local/share/toggle/` and, inside a Git repo, `<repo>/.toggle/`. Each directory's name SHALL identify its feature. No manifest SHALL be required, and discovery SHALL NOT search arbitrary-depth groups.

#### Scenario: Discover without a manifest
- **GIVEN** the global root contains `openspec/source/` and `review/source/`, with no manifests
- **WHEN** `toggle list` runs outside Git
- **THEN** the output lists `openspec` and `review` with their feature directory paths

#### Scenario: Ignore grouping directories
- **GIVEN** the global root contains `group/review/source/` but no `group/source/`
- **WHEN** `toggle list` runs outside Git
- **THEN** neither `group` nor the nested `review` appears as a feature

### Requirement: Select repo-local features before global features

Within a repo, a feature in `.toggle/<name>/` SHALL override a global feature with the same name. Listing and source selection SHALL use the local definition, without changing either definition or reinstalling an already installed feature.

#### Scenario: Select a local override
- **GIVEN** global `review/source/` and repo-local `.toggle/review/source/` contain different skill instructions
- **AND** review is off and its destination is absent
- **WHEN** `toggle on review` runs from the repo root
- **THEN** the installed skill contains the repo-local instructions
- **AND** both feature definitions remain unchanged

#### Scenario: Select the global feature after a local override is removed
- **GIVEN** global `review/source/` remains available after `.toggle/review/` has been removed
- **WHEN** `toggle list` runs inside the repo
- **THEN** it lists review once with its global feature directory
- **AND** it creates no files and changes no installation records

### Requirement: List available features in context

`toggle list` SHALL list available features and their selected directory paths, regardless of whether they are installed. Inside a repo it SHALL include global and repo-local features with local precedence; outside Git it SHALL include global features. A missing discovery root SHALL contribute no features. An empty list SHALL be returned only when no features are available, without creating files.

#### Scenario: List both sources inside a repo
- **GIVEN** global features are `openspec` and `review`
- **AND** the repo contains `.toggle/review/source/` and `.toggle/checks/source/`
- **AND** only openspec is installed
- **WHEN** `toggle list` runs in that repo
- **THEN** the output lists openspec from the global root and review and checks from `.toggle/`
- **AND** review appears once

#### Scenario: List global features outside a repo
- **GIVEN** global `openspec/source/` exists and the working directory is outside Git
- **WHEN** `toggle list` runs
- **THEN** the command succeeds and lists openspec with its global feature directory

#### Scenario: List with no global root
- **GIVEN** the global discovery root is absent and the working directory is outside Git
- **WHEN** `toggle list` runs
- **THEN** the command succeeds with an empty list and creates no files

### Requirement: Mirror source paths into the repo

On SHALL install files under the selected feature's `source/` at the same relative paths in the repo, preserving contents and executable modes. Only `source/` contents SHALL be installed; top-level metadata SHALL remain outside installation. JSON and JSONC files SHALL be copied whole without merging.

#### Scenario: Install the source tree without metadata
- **GIVEN** review contains `README.md` and `source/.agents/skills/review/SKILL.md`
- **AND** the repo's target skill is absent
- **WHEN** `toggle on review` runs from the repo root
- **THEN** `.agents/skills/review/SKILL.md` contains the source instructions
- **AND** the feature's top-level README is not installed

#### Scenario: Copy a JSON file whole
- **GIVEN** review contains `source/settings.json` and the repo has no `settings.json`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** repo `settings.json` has exactly the source file's bytes

### Requirement: Keep paths within their roots

Resolved source files SHALL stay within the selected feature's `source/`. Declared and resolved destinations SHALL stay within the repo, outside Git metadata and `.toggle/`. A boundary violation SHALL fail before changes.

#### Scenario: Reject an escaping source symlink
- **GIVEN** review contains a source file symlink resolving outside its `source/`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** the command fails and identifies the escaping source path
- **AND** source files, repo files, exclusions, and records remain unchanged

#### Scenario: Reject a destination outside the repo
- **GIVEN** `.agents/skills` in the repo is a symlink resolving outside the repo
- **AND** review contains `source/.agents/skills/review/SKILL.md`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** the command fails and identifies the escaping destination
- **AND** no repo files, outside files, exclusions, or records change

#### Scenario: Protect local feature definitions
- **GIVEN** review contains `source/.toggle/other/source/instructions.md`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** the command fails and identifies the reserved `.toggle/` destination
- **AND** repo files, definitions, exclusions, and records remain unchanged

#### Scenario: Protect definitions reached through an alias
- **GIVEN** `definitions` in the repo is a symlink to `.toggle/`
- **AND** review contains `source/definitions/other/source/instructions.md`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** the command fails and identifies the destination inside `.toggle/`
- **AND** the symlink, definitions, exclusions, and records remain unchanged

#### Scenario: Protect Git metadata
- **GIVEN** review contains `source/.git/config`
- **WHEN** `toggle on review` runs from the repo root
- **THEN** the command fails and identifies the Git metadata destination
- **AND** Git metadata, repo files, exclusions, and records remain unchanged

### Requirement: Treat aliases as one destination

Toggle SHALL use resolved destinations for overlap checks, installed-file identity, and exact exclusions. It SHALL preserve existing destination symlink entries and change only the resolved files. Git tracking checks SHALL cover declared and resolved paths.

#### Scenario: Refuse an alias of another feature's installed file
- **GIVEN** checks has installed `.agents/skills/review/SKILL.md`
- **AND** repo `skills-link` resolves to `.agents/skills` and review contains `source/skills-link/review/SKILL.md` with identical contents
- **WHEN** `toggle on review` runs from the repo root
- **THEN** the command fails and reports the shared resolved destination and both features
- **AND** installed files, the symlink, exclusions, and records remain unchanged

#### Scenario: Preserve an internal destination symlink
- **GIVEN** repo `.agents/skills` resolves to `.local-skills` within the repo
- **AND** review contains `source/.agents/skills/review/SKILL.md` and the resolved target is absent
- **WHEN** `toggle on review` runs from the repo root
- **THEN** `.local-skills/review/SKILL.md` contains the source instructions
- **AND** `.agents/skills` remains the same symlink

### Requirement: Refuse redirected installed destinations

Before changing or removing an installed file, toggle SHALL verify that its declared path still resolves to the destination recorded at on. A changed resolution SHALL conflict before changes, even when both destinations are inside the repo.

#### Scenario: Preserve files after a skill symlink is retargeted
- **GIVEN** review was installed through `.agents/skills`, resolving to `.local-skills` within the repo
- **AND** that symlink now resolves to `.other-skills`, containing an unrelated file at the same relative path
- **WHEN** `toggle off review` runs from the repo root
- **THEN** the command fails and reports the declared, recorded, and current destinations
- **AND** both files, the symlink, exclusions, and records remain unchanged
