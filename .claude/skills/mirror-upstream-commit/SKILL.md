---
name: mirror-upstream-commit
description: Bring a landed FreeBSD ports commit into this overlay with git am, preserving the original author and author date. Use when a commit-hook or FreshPorts mail reports a commit to a maintained port, or when checking whether the overlay has fallen behind upstream.
allowed-tools: Bash(gh api *) Bash(fetch *) Bash(git *) Bash(grep *) Bash(cmp *) Bash(base64 *) Bash(mktemp *) Read Grep Glob AskUserQuestion
---

# Mirror an Upstream Commit

The authoritative copy of every port here is the FreeBSD ports tree. This repo
is where changes are prepared before submission and where upstream changes are
mirrored back. Mirroring uses `git am` so the original author and author date
survive and only the committer becomes Douglas.

Triggers: a commit-hook "A commit references this bug" mail; a FreshPorts
watch-list notification; or a periodic check for drift, which is the
mail-independent mode where the sharp edges live.

## Step 1: Find what is missing

Per port, ask upstream for commits newer than the last one this repo has:

```sh
last=$(git log -1 --format='%aI' -- <category>/<port>)
since=$(python3 -c "import datetime,sys;d=datetime.datetime.fromisoformat(sys.argv[1]);print((d+datetime.timedelta(seconds=1)).isoformat())" "$last")
gh api "repos/freebsd/freebsd-ports/commits?path=<category>/<port>&since=$since" \
  --jq '.[] | "\(.sha[0:9]) \(.commit.author.date[0:10]) \(.commit.message|split("\n")[0])"'
```

**Most of what comes back is already mirrored.** GitHub's `since` filters on
**committer** date, which for a ports commit is routinely days later than the
author date the local mirror carries — so a commit you already have looks newer
than your own copy of it. Measured 2026-10-16: 16 of the 22 ports returned hits
and 14 of those were commits already present.

So match on **subject and author date** against local history before concluding
anything is new:

```sh
git log --date=short --pretty='%h %ad %an | %s' -- <category>/<port>
```

**Author date and commit date differ, and you must say which you are using.**
`0d01e25f7` was authored 2026-09-30 17:18:30Z and committed 2026-10-03
09:19:10Z. Quoting one as the other produced a confidently wrong timeline in the
one repo whose entire mirroring convention exists to keep the two apart.

## Step 2: Get the patch

GitHub serves any ports commit as a ready-made mbox:

```sh
fetch -qo - "https://github.com/freebsd/freebsd-ports/commit/<sha>.patch" | git am
```

**That endpoint returns 403 intermittently.** The same commit can succeed and
then be refused a minute later; it does not track patch size or file count, so
it has to be handled rather than assumed away. The piped form fails safely —
`git am` gets an empty stream — but any variant that writes to a file **must**
check the fetch succeeded, or it applies whatever patch was left there last:

```sh
fetch -qo "$sha.patch" "https://github.com/freebsd/freebsd-ports/commit/$sha.patch" \
  || { echo "fetch failed"; exit 1; }
```

The fallback is a local clone of the ports tree:

```sh
git -C <ports-clone> format-patch -1 --stdout <sha> | git am
```

A patch touching ports this overlay does not carry can be filtered — `git am`
passes `--include` and `--exclude` through to `git apply`:

```sh
fetch -qo - "https://github.com/freebsd/freebsd-ports/commit/<sha>.patch" |
    git am --include='dns/rubygem-simpleidn/*'
```

Check the paths first, so you know whether a filter is needed at all:

```sh
grep -E '^(diff --git|new file|deleted file)' <sha>.patch
```

## Step 3: Order matters, and it is not hypothetical

**A patch only applies if the local file matches the context it was authored
against.** Upstream tree-wide sweeps — removing `# $FreeBSD$`, moving `WWW=`
into Makefiles, `USE_RUBY=yes` → `USES=ruby`, man pages to `share/man` — change
that context. So patches must be replayed in upstream **author-date order**, or
the port directory must first be synced to the state the patch expects.

`git am -3` does **not** rescue a mismatch here: the blobs named in the patch's
`index` lines are not in this repo's object store unless the ports tree has been
added as a remote and fetched.

The worked example, mirrored 2026-10-16: `0d01e25f7` ("fix Python library
linking") *creates* `www/mod_wsgi/files/patch-configure.ac`, and `6228cc78a`
(6.1.1) *modifies* that same file. Out of order, the second fails on a file that
does not exist yet. They landed as `d4319e0` and `bf770f8`, in that order.

## Step 4: Preflight

Two properties gate the operation, and `git apply --check` tests only one of
them:

```sh
git status --porcelain            # must be empty of staged changes
git apply --check <sha>.patch     # writes nothing
```

`git am` **refuses outright on a dirty index** — `fatal: Dirty index: cannot
apply patches` — so a clean `apply --check` on a repo with staged work still
fails at the `am`. Check both.

Only the **first** patch of a dependent pair can be `apply --check`ed; the
second's context does not exist until the first lands. So for a series: check,
apply, check the next, apply.

## Step 5: Apply

Apply a dependent series as **one** `&&`-chained command rather than separate
calls, so a failure stops the series instead of stranding the later patches
against a tree the earlier ones never reached:

```sh
git am <first>.patch && git am <second>.patch
```

If it exits non-zero with **no git output at all**, nothing was written — report
that and stop rather than retrying the `am`.

Whitespace warnings from `git am` on a patch that adds a `files/patch-*` are
normal: the embedded patch's own context lines carry trailing whitespace
upstream too.

If an `am` fails partway, the session is left mid-`am`. `git am --abort` is a
stop-and-ask, not a default — recover from the actual state first
(`git status`, `.git/rebase-apply`).

## Step 6: Verify the content, not just the application

**A clean `git am` proves the patch applied, not that the result matches
upstream.** Compare every file in the port directory against upstream's blobs at
the commit you mirrored:

```sh
tmp=$(mktemp -d)
for f in $(git ls-files <category>/<port>); do
  gh api "repos/freebsd/freebsd-ports/contents/$f?ref=<sha>" --jq '.content' \
    | base64 -d > "$tmp/up.$$"
  cmp -s "$f" "$tmp/up.$$" && echo "same    $f" || echo "DIFFERS $f"
done
```

Byte-identical across the whole directory is what settles it. This matters most
where the local file has diverged from upstream for a reason — a clean apply on
a structurally different file can land the intended content twice, or in the
wrong place.

## Step 7: CI on push

Caronade derives the affected ports from each commit's **changed file paths** —
the first two segments of any added or modified path at least three deep. Jobs
are generated per commit, on the queues in `default_queues`.

So every mirrored commit generates jobs, and **mirroring in bulk matches en
masse**. Upstream commits have already been built by the FreeBSD cluster, so
there is nothing for CI to add. Before a bulk import, either disable the webhook
for the duration or put `CI: no` in the messages — the tag must start its line,
and is matched case-insensitively with `no` tested before `yes`.

Measured on the 2026-10-16 push of four commits: two builds, one per
port-touching commit, each with a `14.4/amd64` and a `portlint` job; the two
root-level-only commits generated nothing. For one or two commits that is cheap
enough to leave alone — ask the user rather than deciding, since rewriting an
upstream message to insert `CI: no` is a change to the thing being mirrored.

Expect one spurious `portlint` warning on a mirrored commit that sets
`PORTREVISION`: caronade's recipe runs `portlint -A` unconditionally, and new-port
mode objects to `PORTREVISION` on principle. The Rakefile picks `-C`/`-A` by
whether the port exists in `githead`; caronade's recipe does not.

## Never hand-edit instead

Mirroring is `git am` precisely so authorship survives. Hand-applying an
upstream change makes Douglas the author of someone else's work, and the commit
then misreports who wrote it. If a patch will not apply, fix the ordering or
sync the directory — do not retype the change.

Mirrored commits also keep upstream's own trailers (`PR:`, `Submitted by:`,
`Reviewed by:`, `Approved by:`) exactly as they came. **Never add
`Co-Authored-By: Claude` or `Claude-Session`** — see this repo's `CLAUDE.md`;
those would leak into an upstream submission.

## Common Mistakes

- **Treating `gh api … since=<author date>` results as new commits.** `since`
  filters on committer date; match subject and author date against local history.
- **Writing the patch to a file without checking the fetch.** A 403 leaves the
  previous patch in place and you apply that instead.
- **Reaching for `git am -3` when a patch conflicts.** The index blobs are not
  in this object store; the problem is ordering or context, not merge strategy.
- **Concluding a mirror is correct because `git am` exited 0.** Compare against
  upstream's blobs.
- **Mirroring a dependent pair in message order rather than author-date order.**
  Check which commit creates a file the other modifies.
- **Splitting a dependent series across separate tool calls.** Anything that
  must hold between the patches — an authenticated state, a precondition — can
  lapse in the gap, stranding the later ones. One chained command.
- **Building a conclusion from output sliced with `head`/`tail`.** And an empty
  `grep` cannot distinguish "absent" from "the command died" — check the exit
  status alongside filtered output.
