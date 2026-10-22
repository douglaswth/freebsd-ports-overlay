---
name: update-port
description: Bump an existing FreeBSD port in this overlay to a new version, or deprecate one, then test and submit the patch to Bugzilla. Use when portscout reports a port out of date, when pkg-fallout reports a build failure, when a security advisory lands, or when the user asks to update a port.
allowed-tools: Bash(bugzilla query *) Bash(bugzilla attach *) Bash(bugzilla new *) Bash(bugzilla modify *) Bash(portedit *) Bash(gh api *) Bash(fetch -qo - *) Bash(bundle exec rake *) Bash(make *) Bash(git *) Read Edit Write Grep Glob WebFetch WebSearch AskUserQuestion Agent
---

# Update a Port

Bump a port this overlay carries. Triggers, in order of how often they fire: a
portscout "ports you maintain which are out of date" mail, which carries the
new version; a pkg-fallout failure; a security advisory; a direct ask.

If the job is to verify somebody *else's* patch, that is `review-port-pr`. If
the port does not exist yet, `add-port`.

## Step 1: Check whether an update is already pending

Do this before any work. `upgrading` is explicit: *"The next step is to see if
there is an update already pending… Select `Ports & Packages` in the `Product`
multiple select menu, and enter the name of the port in the `Summary` field."*

```sh
bugzilla query --product "Ports & Packages" --summary <portname>
```

Somebody may have submitted the same bump already, in which case the work is a
`review-port-pr` on their patch instead. Also check upstream, since a committer
may have done it without a bug:

```sh
gh api "repos/freebsd/freebsd-ports/commits?path=<category>/<port>" \
  --jq '.[0] | "\(.sha[0:9]) \(.commit.author.date[0:10]) \(.commit.message|split("\n")[0])"'
```

If upstream is ahead of this overlay, the work is `mirror-upstream-commit`, not
an update.

## Step 2: Read the upstream release notes

Every time, before editing anything. Nothing in a version number says
"security", and nothing in a diff says "this release changed where config files
live". What to extract:

- A security advisory — if there is one, jump to **Security updates** below,
  because the deliverable count changes from one to three.
- Behaviour changes that need an `UPDATING` entry.
- Requirement floors that moved (step 5).
- Whether a fix upstream makes an existing `files/patch-*` redundant. A patch
  that no longer applies is a build failure; a patch that still applies but is
  now unnecessary is silent cruft.

## Step 3: Make the change

```sh
portedit set-version -i <version>
```

**`-i` is required.** Without it `portedit` prints the edited Makefile to stdout
and leaves the file untouched — a silent no-op that looks like success. The same
goes for `bump-revision` and `bump-epoch` below; only the read-only
`unknown-vars` / `unknown-targets` need no flag.

`portedit` knows whether the port uses `DISTVERSION`, `DISTVERSIONPREFIX` or
`PORTVERSION` and edits the right one — do not guess. Then:

```sh
bundle exec rake makesum
```

`CHECKSUM_ALGORITHMS` is `sha256` alone, so that is what `distinfo` gets.
Before accepting it, check the fetched distfile against the hash the project
**publishes** — `rake sha256` (or `md5`/`sha1`/`sha512`, whichever they publish)
prints sums of the fetched distfiles for exactly this comparison. A hash that
matches what you downloaded proves nothing about what upstream released.

**`PORTREVISION`**, by the `security` chapter's rule: bump it *"if adding a
patch file to the port, but do not bump it if updating the port to the latest
software version and thus already touched `DISTVERSION`"*. So a packaging-only
change bumps it; a version bump drops it — and `set-version -i` removes it for
you, verified on a `PORTVERSION` port. Use
`portedit bump-revision -i`, which sanitizes the variable rather than
hand-editing it — and `portedit bump-epoch -i` for the rarer `PORTEPOCH` case,
when a new version sorts *lower* than the old one.

## Step 4: `USES` changes are lookups, not judgements

Any `USES` line you touch gets checked against the `uses` chapter — 3,221 lines
of reference table, not narrative. It is the chapter this skill consults most.

Known live case: `USES=gettext` is deprecated there in favour of
`gettext-runtime` / `gettext-tools`, and which one is right depends on whether
the build runs `msgfmt`. `sysutils/psmisc` still carries the deprecated form via
`NLS_USES= gettext`.

Where the decision turns on what the framework will actually *do* rather than
what the chapter says, read `Mk/` — that is the handbook's own policy, from
`new-port`: *"When this document is not sufficiently detailed, refer to
/usr/ports/Mk/bsd.port.mk"*, and *"Only a fraction of the variables (`_VAR_`)
that can be overridden are mentioned in this document."*

**Name the tree and the date in any `Mk/` claim.** Two trees disagree here:
`/usr/ports` (the `default` poudriere tree) and
`/usr/local/poudriere/ports/githead`. On 2026-10-16 they held the same
`autoplist` condition 11 lines apart, and `githead` still listed Python 3.10 in
`_PYTHON_VERSIONS` where `/usr/ports` had dropped it. `rake update_githead`
refreshes `githead` on the first task of a day that depends on it, and
`portclippy`/`portfmt` carry no such prerequisite, so a day on those leaves it
weeks behind.

## Step 5: Did a requirement floor move?

Both `USES=python` and `USES=apache` take version arguments (`python:3.11+`,
`apache:2.4+`) and that is the documented way to declare a minimum. A moot
constraint costs nothing; an undeclared one silently stops being moot when the
tree moves.

Resolve what exists by evaluating, never by reasoning: `make -V FLAVORS` for
flavors, `_PYTHON_VERSIONS` in the named tree for Python versions.

## Step 6: Lint

```sh
bundle exec rake portclippy
bundle exec rake portfmt
bundle exec rake portlint
```

`portlint` takes `-C` for an existing port, and a recent `portlint` runs
`make describe` itself, so that check is covered.

All three of `portlint`, `diff` and `makepatch` depend on `update_githead`,
which runs `sudo poudriere ports -u` on the first task of any day. So the first
of these in a day wants Douglas's sudo — `portclippy` and `portfmt` do not,
having no such prerequisite.

Expect `portclippy` and `portlint` to disagree on variable order — run both,
reconcile by hand, treat neither as authoritative. And do not expect
`rake portfmt` to fix a `portclippy` complaint: it aligns tabs and sorts tokens
*within* a variable, never reorders variables. Where it does act it re-sorts
dependency lists, which lands in a submitted diff as unrelated churn — so check
what it changed before keeping it.

## Step 7: Run the tests

The `testing` chapter: *"Please make sure that tests do not break when updating
a port."*

Run `make test` wherever the port has a test target **at all**. The chapter
describes one route, `TEST_TARGET` (with `TEST_WRKSRC`, `TEST_ENV`,
`TEST_ARGS`), but `Mk/` adds a second it never mentions, and a port here takes
it: `security/py-keyrings.alt` has an empty `make -V TEST_TARGET`, and its test
target comes from `USE_PYTHON=pytest` causing `python.mk` to define `do-test`.
Keying on `TEST_TARGET` alone would report that port as having no tests.

## Step 8: Build it

Two routes, in this order.

**`rake testport` first, and it is Douglas's to run** — it needs sudo, which
fails in a Claude session. Hand him the command; do not gate on it:

```sh
rake testport
```

It builds against this working copy through poudriere's nullfs overlay mount,
so it tests uncommitted work, which is the whole point at this stage. Useful
variants to mention when a build fails: `-i` enters the jail for interactive
testing and cleans up automatically on exit, `-I` is the one that **leaves** the
jail running for `jexec`, and `-c` runs `make config` for the port first. Where options alter `pkg-plist`, the chapter asks for a run with all
options selected and one with all deselected.

**Then the full matrix, once local tests pass.** Push with `CI: yes` in the
commit message, which forces every caronade queue rather than the two in
`default_queues` — that sweeps `14.4/arm64`, which a default push never touches.
Claude can drive and wait on that end to end.

A note on which jails *should* exist, since the answer is in `Mk/` rather than
folklore — and this is a live demonstration of why a `Mk/` claim must name its
tree, because **the two trees disagree on this exact line**:

- `/usr/ports/Mk/bsd.port.mk:1170` (2026-10-16) declares a system unsupported
  when `OSVERSION < 1404000 || (OSVERSION >= 1500000 && OSVERSION < 1501000)` —
  two intervals, so 14.4+ and 15.1+ are supported and 15.0 is explicitly out.
- `githead`'s copy of the same line (tree dated 2026-09-29) is just
  `OSVERSION < 1404000`, with no 15.0 exclusion at all.

`githead` is the tree `rake testport -p githead` actually builds against, so
read the predicate from the tree you are about to use and say which one it was.
Either way the configured jails are `144amd64` and `144arm64`, both
14.4-RELEASE-p11, so there is **no 15.1 jail** and the "one jail per live
branch" goal is unmet. Surface that; do not quietly skip it.

## Step 9: Produce the diff

```sh
bundle exec rake 'diff[patch]'
```

That writes `<PKGNAME>.diff` at the repo root, which `.gitignore` covers. It
compares this working copy against the ports tree's copy with
`git diff --no-index`, rewritten to the `a/`/`b/` form a submitted diff carries.

**`git diff` in this repo answers a different question** — it compares against
this repo's own history, so once a change is committed here it shows nothing,
and it never shows what differs from what FreeBSD ships.

## Step 10: Independent review

Once `testport` passes, and before anything goes to Bugzilla, dispatch a fresh
reviewer with the `Agent` tool (`general-purpose`). Every check so far either
ran a tool or ran past your own judgement; a reviewer with none of this
session's context reads the diff the way a committer will.

**Brief it with artifacts, not conclusions.** Give it the `<PKGNAME>.diff`
path, the port directory, the old and new versions, upstream's release notes
or changelog for that range, the trigger (portscout mail, pkg-fallout log,
advisory id), and the Porter's Handbook at
<https://docs.freebsd.org/en/books/porters-handbook/>. Do **not** give it your
reasoning — why no `UPDATING` entry, why a patch stays, why `PORTREVISION` went
the way it did. A justification in the brief is the reviewer adopting your
blind spot instead of finding it.

Ask it to:

- read-only: no edits, no `rake` targets that need `sudo`, no Bugzilla writes;
- check the diff against the release notes (behaviour changes needing
  `UPDATING`, moved requirement floors, `files/patch-*` made redundant), against
  the handbook, and — for a security update — against the three deliverables
  below, citing the chapter or source for every finding;
- report what it checked and found sound as well as what it found wrong, so
  silence on an area means "not examined", never "fine".

**Its findings are claims, not verdicts.** Verify each against the source it
cites before acting, and put the verified list in front of Douglas with what
you propose for each. A change that touches the port reruns steps 6 to 9 —
`testport` included — **and then this step**, with a fresh reviewer and a
freshly written diff, before submission. A changed patch, a bumped dependency
or a rewritten `UPDATING` entry is exactly the kind of choice the earlier
reviewer never saw; passing lint and `testport` says nothing about whether it
is right.

## Step 11: Submit

**Commit first, then submit.** Once Douglas has seen the verified review and
the change is final, commit it here — `git format-patch` needs a commit, and
the attachment is generated from it, not from the working tree. Generate it
with `git format-patch -1 --stdout <commit>` into the session scratchpad, not
the repo root, where `.gitignore` covers `*.diff` but not `*.patch`. **Compare it
with step 9's `rake diff[patch]` before attaching.** The commit diffs against
this repo's previous state, `rake diff` against what FreeBSD ships; they agree
only if this port was in sync with the tree before the change. If they differ,
the port here was behind or ahead of the tree — mirror the missing upstream
commits first (see this repo's `CLAUDE.md`) rather than submitting a patch
that will not apply.

Title: `category/portname: Update to X.Y`, or `category/portname: add FOO
option` for an option change. Product "Ports & Packages", Component "Individual
Port(s)".

From the Problem Reports article, which is where `upgrading` defers submission
mechanics and which specifies things the handbook does not:

- **Patches as attachments**, not inline — mail clients render tabs as spaces,
  *"which will completely ruin anything intended to be part of a Makefile."*
- **MIME type `text/plain`**, and never
  `Content-Transfer-Encoding: quoted-printable`, which escapes characters and
  renders the patch useless.
- **`git format-patch` is preferred** over a plain diff, based at the base of
  the appropriate tree, because it carries the author's name and email so a
  committer can `git am` it and credit correctly.
- **Severity honestly**: *"do not overreact; refrain from labeling your problem
  `Affects many people` unless it really does."*
- Name added or deleted files explicitly, and do not compress or encode the
  diff. Those two are the `upgrading` chapter's, not the article's: *"Please
  mention any added or deleted files in the message, as they have to be
  explicitly specified to git when doing a commit"* and *"Do not compress or
  encode the diff"*.
- A submitted patch is assumed licensed under the same terms as the file it
  modifies unless stated otherwise.

The CLI has no `--patch` flag, so the two attachment properties a correct ports
patch carries are set explicitly:

```sh
bugzilla attach --type text/plain --field is_patch=1 --file <file> <bug>
```

## Security updates have three deliverables, not one

1. **The port change**, with `PORTREVISION` per step 3.
2. **A VuXML entry.** `pkg audit` reads VuXML and nothing else, so without an
   entry the vulnerability is invisible to every user's `pkg audit`. The
   `security` chapter gives this four sections, including a new-entry checklist
   and how to test a change. Observed cost of skipping it: GHSA-q67r-wxfm-97hj
   was published 2026-10-13 and the VuXML entry landed 2026-10-16, only because
   it was asked for on the PR — `pkg audit` reported `0 problem(s)` on an
   affected installed package throughout.
3. **A portmgr notification**, to *"request immediate rebuilding and
   redistribution of the port's package"*, because *"Unsuspecting users of
   `pkg` will otherwise continue to install the old version via `pkg install`
   for several weeks."* Both quotes are `upgrading`'s, and its trigger is
   broader than security alone: *"If the upgrade is motivated by security
   concerns or a serious fault in the currently committed port"* — so a bad
   build that shipped qualifies too.

Also put `Security: <advisory id>` in the commit message, and ask for an MFH to
the current quarterly branch.

## Deprecating a port

- `EXPIRATION_DATE` in ISO 8601, and it is meaningless without `DEPRECATED`.
  `DEPRECATED` alone is valid.
- Name any replacement port in the commit message.
- Notice period, from `porting-dads`: *"one month for security-related issues
  and two months for build issues."* `www/dtse` was deprecated 2026-08-29 to
  expire 2026-09-30 — 32 days — for "Unmaintained upstream", which is neither
  case; it was reverted three days later, so nothing turned on it.

## Messages sent in Douglas's name

Every Bugzilla comment, bug and attachment description that goes out as him is
**approved or reshaped first, and the gate is not yes/no.** Show the draft in
full, exactly as it will render. Use `AskUserQuestion` with options that offer
reshaping rather than send/hold, treat the free-text answer as the edit channel,
and repeat until he approves. A summary of a draft is not a draft.

Comments render inside `<pre class="bz_comment_text">`: **no Markdown**,
whitespace literal, so **hard-wrap it** — ~76 columns by convention, the
width FreeBSD's own comments use. References use the `[N]` convention — a bare
`[1]` inline with a matching trailer block — which is FreeBSD's idiom and needs
no markup.

## Commits in this repo

Commits here feed patches submitted upstream, so they follow FreeBSD ports
conventions. **Never add `Co-Authored-By: Claude` or `Claude-Session`
trailers** — see this repo's `CLAUDE.md`.

A commit touching a port directory generates caronade jobs on push. Upstream
commits mirrored in need `CI: no`; your own changes want the build, and
`CI: yes` forces all queues.

## Common Mistakes

- **Skipping step 1.** Duplicating a bump somebody already submitted wastes the
  work and the committer's time.
- **Deriving the new version from the portscout mail alone.** It is a crawler,
  and it can report a version upstream never released or miss a newer one. Check
  upstream's own release page.
- **Keying tests on `TEST_TARGET`.** See step 7 — a `USES`/`USE_PYTHON` feature
  can define `do-test` with `TEST_TARGET` empty.
- **Expecting `rake portfmt` to fix `portclippy`.** It does not reorder
  variables, which is what `portclippy` reports.
- **Claiming a `Mk/` fact without naming the tree and date.** The two trees
  disagree by weeks of commits.
- **Treating a clean `pkg audit` as evidence a package is safe.** It reports
  what the local VuXML copy knows; the copy lags, and the entry may not exist.
- **Briefing the reviewer with your reasoning.** Step 10's reviewer gets the
  diff and the sources, not why you chose what you chose — or it reviews your
  argument instead of the port.
- **Using `git diff` to produce the submission diff.** `rake diff[patch]` is the
  one that answers what a PR asks.
- **Building a conclusion from output sliced with `head`/`tail`.** And an empty
  `grep` result cannot distinguish "absent" from "the command died" — check the
  exit status alongside filtered output.
- **Re-running a `rake` target after editing the Rakefile without re-running
  `rake -T`.** A check that passed before a later edit is worthless.
