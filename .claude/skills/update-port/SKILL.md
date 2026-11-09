---
name: update-port
description: Bump an existing FreeBSD port in this overlay to a new version, or deprecate one, then test and submit the patch to Bugzilla. Use when portscout reports a port out of date, when pkg-fallout reports a build failure, when a security advisory lands, or when the user asks to update a port.
allowed-tools: Bash(bugzilla query *) Bash(bugzilla attach *) Bash(bugzilla new *) Bash(bugzilla modify *) Bash(portedit *) Bash(gh api *) Bash(fetch -qo - *) Bash(bundle exec rake *) Bash(make *) Bash(git *) Read Edit Write Grep Glob WebFetch WebSearch Bash(pgrep *) AskUserQuestion Agent Monitor
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
- Behavior changes that need an `UPDATING` entry.
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

## Step 4: `USES` changes are lookups, not judgments

Any `USES` line you touch gets checked against the `uses` chapter — 3,221 lines
of reference table, not narrative. It is the chapter this skill consults most.

Known live case: `USES=gettext` is deprecated there in favor of
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
of these in a day wants the user's sudo — `portclippy` and `portfmt` do not,
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

**`rake testport` first, and it is the user's to run** — it needs sudo, which
a Claude session may not allow. Hand them the command; do not gate on it:

```sh
rake testport
```

It builds against this working copy through poudriere's nullfs overlay mount,
so it tests uncommitted work, which is the whole point at this stage. Useful
variants to mention when a build fails: `rake 'testport[-i]'` enters the jail
for interactive testing and cleans up automatically on exit,
`rake 'testport[-I]'` is the one that **leaves** the jail running for `jexec`,
and `rake 'testport[-c]'` runs `make config` for the port first. Any argument
starting with `-` is passed to `poudriere testport`, and the arguments go in
any order, so a jail, and a flavor, which must start with `@` (a name without
one is taken as the jail), can go alongside a flag, as in
`rake 'testport[@<flavor>,-i]'`. Where options alter `pkg-plist`, the chapter
asks for a run with all options selected and one with all deselected.

**Follow every build with `Monitor` from the moment it starts** —
`rake testport` and caronade (step 11) alike. A loop that waits only for the
result says nothing until the end. Say what each event shows as it arrives. A
monitor lasts at most 30 minutes, so re-arm it until the build ends.

- **caronade:** poll the job's JSON every 30 seconds or more, letting a failed
  fetch pass, and emit each queue's `Status` as it changes, `portlint`
  included. A queue's log exists only once it is `building`; before that,
  fetching it gives "Not Found". For each queue that builds, not `portlint`,
  emit new log lines matching
  `^\[[0-9:]+\] (\[[0-9]+\] \[[0-9:]+\] )?(Building|Finished) `,
  `build failure encountered`, `Build failed in phase` or `build time:`, keeping
  the last line number read so a re-armed monitor continues from there.
  `Building with flags:` means the port's own build has begun; the port gets no
  `Building <origin>` or `Finished` line. When a queue's `Status` becomes
  `success` or `failure`, read its log once more before emitting the `Status`;
  it is the result. A job that dies early, such as on the clone, gives the
  monitor nothing to emit but the `Status`. Stop when no queue is `waiting` or
  `building`. An empty `Build` means no queue ran, not that every queue passed.
- **`rake testport`:** it shows you no output, and its runs are under
  `/usr/local/poudriere/data/logs/bulk/<jail>-githead/`. The Rakefile picks the
  jail matching this host's version and architecture, ignoring the patch level
  (`-pN`), unless one is passed as `rake 'testport[<jail>]'`. `latest` there
  points at the previous run until the new one starts, so record it when
  handing over the command.

  Below, `<spec>` is the port's origin, followed by `@<flavor>` when a flavor
  was passed; poudriere writes it that way in its status lines and in
  `.poudriere.ports.tobuild`. The run's process matches
  `pgrep -f 'testport\.sh .*-j <jail> .*-o <spec>( |$)'`, where `<jail>` is
  the one whose `latest` was recorded and any `+` in the origin is written as
  `\+`; naming the jail keeps a run of the same port on another jail from
  matching. Emit a line when that process appears and when it goes away:
  a run that fails before it starts, on a broken or nonexistent port or with
  "jail already running", ends without `latest` ever changing. If neither the
  process nor a new `latest` has appeared when the first monitor expires, ask
  the user what their terminal shows rather than re-arming.

  When `latest` changes, the new run is this one. Once the run directory's
  `.poudriere.status.journal%` has any line after `load_priorities:`
  (`balancing_pool:`, or `committing:` when nothing needs building), emit
  `.poudriere.ports.tobuild`: it lists every package the run will build, and
  a dependency in it may be one the user did not expect. It is written after
  `load_priorities:`, so reading it sooner can find it missing or empty.

  Then emit new `testing:`, `interactive:` and `stopped:` lines from that
  journal, new `starting:` lines from the builders'
  `.poudriere.status.<NN>.journal%` files, new lines in
  `.poudriere.ports.built`, and any entry appearing in `logs/errors/` (they are
  symlinks). Each `starting:` line marks one dependency beginning to build. The
  builders' files appear only if something is built, and the others may not
  exist yet just after `latest` changes. Keep the run directory's name and the
  line counts read, so a re-armed monitor continues from there.

  The build has ended at any `stopped:` line except `stopped:<spec>:…`, which
  success writes just before `done:` and `stopped:done:`. Only `stopped:done:`
  is success. When the process goes away, read the journal once more, since
  its last lines are written just before the process exits. If there is still
  no `stopped:` line, the run died without one, so tell the user, unless the
  last status is `interactive:`. If a monitor expires with no events while the
  process is still running, say so and re-arm.
- **`rake 'testport[-i]'` and `rake 'testport[-I]'`:** both write
  `interactive:` after the port's build, whether it passed or failed. `-i`
  then waits until the user leaves the jail, and ends as above. `-I` leaves the
  jail running and exits at once without a `stopped:` line, so `interactive:`
  with no process left is the end of a `-I` run; an empty `logs/errors/` means
  the build passed. The jail stays up until
  `sudo poudriere jail -k -j <jail> -p githead` stops it, and until then the
  next `rake testport` on that jail fails with "jail already running", so
  hand the user that command once they are done with the jail.
- **`build time:` is not success.** poudriere prints it on failure too, one line
  before `build failure encountered`.
- **Running long:** this port's earlier `testport` runs and caronade jobs are
  the baseline, once there are any. They are the run directories under
  `/usr/local/poudriere/data/logs/bulk/<jail>-*/` whose
  `.poudriere.status.journal%` has `testing:<spec>`; each one's
  `.poudriere.started` and `.poudriere.ended` hold its start and end times in
  seconds since the epoch. A run well past them is usually building
  dependencies the user may not have expected, often because no binary package
  matched. Say which ones, from `.poudriere.ports.tobuild` and the `Building`
  or `starting:` lines, and how long each took before (see below).

Read poudriere's log for the package that is building, whether that is the port
or one of its dependencies. caronade's job log carries the port's own build as
it happens, but for a dependency only a `Building <origin> | <pkgname>` line and
later a `Finished` line; that `<pkgname>` names the log to read. `rake testport`
shows you no output at all, but each builder's `starting:` line names the
`<pkgname>` the same way. Where poudriere's logs are readable, each package's
own log is `<run>/logs/<pkgname>.log`, where `<run>` is the run directory: for
`rake testport`, the one `latest` pointed at once it changed, under
`/usr/local/poudriere/data/logs/bulk/<jail>-githead/`, and for caronade, the
one the `Logs:` line near the top of its job log names. Every package
poudriere builds has `<phase: …>` markers saying which phase its build is in
(`build`, `stage`, `package`; the port under test adds `install` and
`deinstall`), and the log's modification time says whether it is still
writing. Once in `package`, the log writes nothing more until it is done, so a
long quiet spell there is normal.

To estimate how long a dependency will take — before deciding a run is
running long — look up its `build time:` line in earlier logs from the same
jail:

`/usr/local/poudriere/data/logs/bulk/<jail>-*/2*/logs/<PKGBASE>-[0-9]*.log`

`<PKGBASE>` is the package name without its version: the `<pkgname>` from a
`Building` or `starting:` line minus its version, or `make -V PKGBASE` in the
port's directory. It is not `PORTNAME`, since prefixes such as `py312-` are part
of it. The log name carries the version, so the glob also finds builds of
earlier ones. A log with no `build time:` is a package that was downloaded, not
built. Skip any log ending in `!!! build failure encountered !!!`: a failed
run's time is how long it took to fail.

**Then the full matrix, on caronade** — after the commit, in step 11.

Which jails *should* exist comes from this repo's `CLAUDE.md` ("Testing before
submitting"): the oldest supported version from `Mk/bsd.port.mk`, and the
architectures from FreeBSD's platforms page. The trees can disagree on the
`Mk/` line, so read it from the tree you are about to build against (`githead`
for `rake testport`) and say which one it was. Compare the result with the
jails `poudriere jail -l` lists; if a live branch or architecture has no jail,
surface that rather than quietly skipping it.

## Step 9: Produce the diff

```sh
bundle exec rake diff > <scratchpad>/<portname>.diff
```

That compares this working copy against the ports tree's copy, and goes into
the session scratchpad for review. `rake diff[patch]` is for the attachment,
after the commit (step 11), and refuses to run before it.

**`git diff` in this repo answers a different question** — it compares against
this repo's own history, so once a change is committed here it shows nothing,
and it never shows what differs from what FreeBSD ships.

## Step 10: Independent review

Once `testport` passes, and before anything goes to Bugzilla, dispatch a fresh
reviewer with the `Agent` tool (`general-purpose`). Every check so far either
ran a tool or ran past your own judgment; a reviewer with none of this
session's context reads the diff the way a committer will.

**Brief it with artifacts, not conclusions.** Give it the diff's path, the
port directory, the old and new versions, upstream's release notes
or changelog for that range, the trigger (portscout mail, pkg-fallout log,
advisory id), and the Porter's Handbook at
<https://docs.freebsd.org/en/books/porters-handbook/>. Do **not** give it your
reasoning — why no `UPDATING` entry, why a patch stays, why `PORTREVISION` went
the way it did. A justification in the brief is the reviewer adopting your
blind spot instead of finding it.

Ask it to:

- read-only: no edits, no `rake` targets that need `sudo`, no Bugzilla writes;
- check the diff against the release notes (behavior changes needing
  `UPDATING`, moved requirement floors, `files/patch-*` made redundant), against
  the handbook, and — for a security update — against the three deliverables
  below, citing the chapter or source for every finding;
- report what it checked and found sound as well as what it found wrong, so
  silence on an area means "not examined", never "fine".

**Its findings are claims, not verdicts.** Verify each against the source it
cites before acting, and put the verified list in front of the user with what
you propose for each. A change that touches the port reruns steps 6 to 9 —
`testport` included — **and then this step**, with a fresh reviewer and a
freshly written diff, before submission. A changed patch, a bumped dependency
or a rewritten `UPDATING` entry is exactly the kind of choice the earlier
reviewer never saw; passing lint and `testport` says nothing about whether it
is right.

## Step 11: Submit

**Commit first, then submit.** Once the user has seen the verified review and
the change is final, commit it here — the attachment carries the commit's
author, date and message. Generate it with `rake diff[patch]`, which writes
`<PKGNAME>.diff` at the repo root in the form `git format-patch` gives, so a
committer can `git am` it. It refuses to run while the port has uncommitted
changes. **Check its diffstat before attaching:** it diffs against what
FreeBSD ships, so a file this change did not touch means the port here was
behind or ahead of the tree — mirror the missing upstream commits first (see
this repo's `CLAUDE.md`) rather than submitting a patch that undoes them. If
the change spans more than one commit here, `rake 'diff[patch,<commit>]'`
takes the header from the one named.

**Then push, and wait for caronade before drafting anything.** The push builds
the commit on every queue in `default_queues`; `CI: yes` in the message forces
every queue, including the emulated jails a default push never touches. Each
port in the commit becomes a caronade job, whose page is
`https://caronade.douglasthrift.net/builds/<job id>/`, with the full logs.

**Poll the job's JSON, not Gitea's commit statuses.**
`https://caronade.douglasthrift.net/logs/<job id>/jobdata.v1.json` lists every
queue the job was given under `Build`, each with a `Status` of `waiting`,
`building`, `success` or `failure`. Gitea only ever shows the finished ones:
caronade posts `building` when a queue starts, which is not a state Gitea's
API accepts, and never checks the response, so the rejection is silent. A
queue that crashes caronade's worker is marked `failure` in the JSON and posts
nothing to Gitea at all. Gitea is still where the job id comes from — the
first finished status (`portlint` takes seconds) links to the job page in its
`target_url`. Follow it with `Monitor` as step 8 describes, until no
queue is `waiting` or `building`. Do not draft the report until every queue has
passed. A failure is a change like any other — fix it, and the port goes back
through steps 6 to 10.

**Start that `Monitor` as soon as the job id is known**, not once the build
seems slow. The job log is
`https://caronade.douglasthrift.net/logs/<job id>/<ID>.log`, with `<ID>`
from the queue's entry under `Build`.

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
bugzilla attach --type text/plain --field is_patch=1 \
  --description '<summary>' --file <file> <bug> < <file>
```

Bugzilla's create-bug API takes no attachment, so `bugzilla new` files the bug
and the patch follows in this second call, arriving as comment 1 rather than
inside comment 0 as the web form puts it.

**Feed the file on stdin as well as naming it.** Whenever stdin is not a
terminal — and under the `Bash` tool it never is — the CLI reads the
attachment from stdin and takes only the name from `--file`. Without the
redirect it waits on stdin forever, or, with stdin empty, the server refuses
the file as empty.

Pass `--description`: without it the description silently defaults to the
file's basename, which is not the text you had approved.

**Then check what arrived before reporting it done** — the bytes, and the two
properties the CLI sets:

```sh
cd <scratchpad> && bugzilla attach --get <attachment id>
cmp <scratchpad>/<file> <file>
fetch -qo - 'https://bugs.freebsd.org/bugzilla/rest/bug/<bug>/attachment?exclude_fields=data'
```

`--get` never overwrites; it writes `<file>.1` if the name is taken, and says
which name it used. The JSON should show `content_type` `text/plain` and
`is_patch` `1`.

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

## Messages sent in the user's name

Every Bugzilla comment, bug and attachment description that goes out as the
user is **approved or reshaped first, and the gate is not yes/no.** Show the
draft in full, exactly as it will render. Use `AskUserQuestion` with options
that offer reshaping rather than send/hold, treat the free-text answer as the
edit channel, and repeat until they approve. A summary of a draft is not a
draft.

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
