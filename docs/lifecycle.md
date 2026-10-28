# Turning features on and off

Run on/off at the Git repo root:

```sh
toggle on review --dry-run
toggle on review
toggle status
toggle off review --dry-run
toggle off review
```

On checks the entire operation before adding exact local exclusions, saving installation bookkeeping, and copying source files. A matching untracked file can be accepted without prompting; it then belongs to the installation and normal off removes it. A tracked file conflicts even if identical. A file installed for another feature also conflicts even if identical.

Off uses recorded installed paths. Unchanged files can be removed even when the source has changed. Already absent files need no removal. New files added to the source since on are not removal targets. Unrelated files are preserved, including other skills in the same directory. Destination symlinks are retained while their resolved files are installed or removed. A changed destination mapping blocks off.

Repeating on while installed or off while already off succeeds without changes. Repeated on does not refresh definitions, switch to a new local override, restore a missing file, or overwrite edits. To install a changed definition, resolve edits, turn the feature off, then on again.

## Preserve edits explicitly

If installed content or executable bits have changed since on, off requires them to match the same relative path in the currently selected source. Choose where to preserve them yourself:

- Update the global source when the change should be shared, provided no local override is selected.
- Create or update `.toggle/<name>/source/` when the change should stay with this repo. A local feature replaces the global definition of that name; it is not an overlay. Include the files you want the local feature to install.

For example, after editing `.agents/skills/review/SKILL.md`, save the same bytes and executable mode to `.toggle/review/source/.agents/skills/review/SKILL.md`. Off can then remove the installed copy, and the next on installs the local source. The repo owner chooses whether to commit or ignore `.toggle/`.

Toggle does not save an edit cache, update source definitions, or generate diffs. It reports the feature, conflicting path, and reason so an agent or human can use their own comparison tools. If no global or local definition remains, cleanup is manual.

## Validation conflicts and write failures

Known conflicts stop the whole operation before persistent changes. A preview uses the same checks and changes nothing.

Unexpected write failures can leave partial effects. On establishes exclusions and bookkeeping before writing files; off removes files before exclusions and bookkeeping. After a failure, later writes stop and completed effects stay in place. The failed write itself may also have partial effects.

The error reports completed effects, the failed path, remaining effects, and bookkeeping/exclusion locations. Inspect those paths and clean up manually. There is no automatic rollback or repair command, and repeated on is not a retry mechanism.
