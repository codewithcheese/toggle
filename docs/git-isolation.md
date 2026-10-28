# Git isolation and installation records

On/off and their previews require the Git working-tree root. List and status can locate that root from subdirectories; status requires a Git repo, while list also works outside Git.

Toggle checks both declared paths and resolved destinations against Git's index before changing installed files or adding exclusions. A matching tracked file still conflicts. Force-adding an installed file with `git add -f` makes off conflict until its tracked state is resolved.

Each installation adds exact file rules in Git's local exclude file, normally `.git/info/exclude`. It leaves versioned ignore files, user rules, and other features' rules intact. A higher-priority `.gitignore` negation that defeats the proposed exclusion is reported before any changes. It never ignores a whole shared skills directory.

`.gitignore` installation targets and `.git` entries or their contents are reserved at every depth, for both declared and resolved paths. They conflict even if untracked or matching. This keeps a feature from changing Git metadata or installing ignore rules that defeat its own exclusions.

Installation bookkeeping is one JSON file in Git-local metadata, normally `.git/toggle.json`. Git resolves both locations, including when `.git` is a file pointing to a separate Git directory. Entries record the selected source directory at on, installed relative paths, declared/resolved destinations, content hashes, executable bits, and the installation's exclusion block. File contents and edited copies are not stored there. Successful off removes the feature's entry.

These records are outside project files and ordinary staging. Repo-local `.toggle/` definitions remain under the owner's control: commit them or ignore them with ordinary Git configuration. Toggle does neither automatically.
