---
name: add-port
description: Create a new FreeBSD port from scratch in this overlay, test it, and submit it to Bugzilla as a new-port PR. Use when the user wants to package software that has no port yet.
allowed-tools: Bash(bugzilla query *) Bash(bugzilla attach *) Bash(bugzilla new *) Bash(bugzilla modify *) Bash(portedit *) Bash(gh api *) Bash(fetch -qo - *) Bash(bundle exec rake *) Bash(make *) Bash(cp *) Bash(git *) Bash(grep *) Read Edit Write Grep Glob WebFetch WebSearch AskUserQuestion Agent
---

# Add a New Port

Package software that has no port yet. **No mail triggers this** — nothing
arrives telling you to package something — which makes it the outlier among the
port skills, and means it always starts from a direct ask.

## Step 1: Does it already exist, or is someone already doing it?

```sh
bugzilla query --product "Ports & Packages" --summary <name>
gh api "search/code?q=<name>+repo:freebsd/freebsd-ports+filename:Makefile" --jq '.items[].path'
```

A port under a different name or category is easy to miss. **Do not list a
category directory to check** — GitHub's contents API caps a listing at 1,000
entries and returns no error, while `www` alone has 2,816 ports in `/usr/ports`
as of 2026-10-19 — so everything past roughly `p5-HTML-W…` is invisible and the
failure mode is an empty `grep`.
The local `githead` tree is the reliable list:

```sh
grep -rn '^    SUBDIR += ' /usr/local/poudriere/ports/githead/<category>/Makefile | grep -i <name>
```

## Step 2: Settle the package name

**This is the check the other skills do not need: `make -V PKGBASE`.** The
handbook's binding requirement is on the **package name**: *"it is quite normal
for several ports to have the same `PORTNAME`, as the `www/apache*` ports do."*
Note it also says elsewhere to *"check that there is not already a port with the
same `PORTNAME`"* — read that as a prompt to look, not as a second uniqueness
rule, since a shared `PORTNAME` with a distinct `PKGBASE` is explicitly fine.

So a taken name is neither a blocker nor a reason to rename the software. The
remedy is `PKGNAMEPREFIX` or `PKGNAMESUFFIX`, and *"do not make them part of
`PORTNAME`"*. Two working patterns in reach:

- `devel/gitea-act_runner` — `PORTNAME=act_runner`, `PKGNAMEPREFIX=gitea-`, and
  the directory named for the resulting `PKGBASE`.
- `www/mod_wsgi` — `PKGNAMEPREFIX=${APACHE_PKGNAMEPREFIX}${PYTHON_PKGNAMEPREFIX}`,
  computed rather than literal.

Version string rules, from `makefiles`: no hyphen anywhere in `PORTVERSION`;
`alpha`/`beta`/`rc`/`pre` reduce to a first letter after a period, as `1.0b2`;
dates as `d_yyyy.mm.dd`, letter-prefixed so a later numbered release still sorts
above them. The hyphen rule has one exception, the string `pl`.

## Step 3: Write the Makefile

`porting-samplem` is the canonical skeleton — read it for the callouts, not as a
template to copy, since the sample itself is dated (`MASTER_SITE_XCONTRIB`,
`IS_INTERACTIVE`; it does use `DISTVERSION`). Four of its callouts are rules:

- **`MAINTAINER` is mandatory**, and *"we do not accept new ports that are
  assigned to `ports@FreeBSD.org`"*.
- **`LICENSE` "should not be empty"** — with `LICENSE_FILE` where upstream ships
  one.
- The first block's order is fixed: `PORTNAME` and
  `PORTVERSION`/`DISTVERSION*` first, then `CATEGORIES`, `MASTER_SITES`,
  `MASTER_SITE_SUBDIR`, then `PKGNAMEPREFIX`/`PKGNAMESUFFIX`, then `DISTNAME`,
  `EXTRACT_SUFX`/`DISTFILES`, `EXTRACT_ONLY`.
- **`OPTIONS_SUB`** when options change the file list.

`USES=cpe` carries a prohibition — *"do not ever make up CPE data"* — and a
procedure: find the official entry in the NVD dictionary, compare
`make -V CPE_STR` against it, then override `CPE_PRODUCT`, `CPE_VENDOR`,
`CPE_VERSION` or `CPE_UPDATE` one field at a time.

Catch typos that would otherwise do nothing silently:

```sh
portedit unknown-vars
portedit unknown-targets
```

## Step 4: Go ports

The rules, with their actual sources — they are **not** all in one Go section:
`GO_MODULE` and `GO_TESTTARGET` are in the `uses` chapter's `go` entry, the
library prohibition and dependency preference are `special`'s top-level
`Go Libraries` section, and completion files are `special`'s
`Shell Completion Files` — a general ports rule, not a Go one.

- `USES=go:modules` with `GO_MODULE` is usually the whole of it: *"in most
  cases, this is the only required variable for ports that use Go modules."*
- **`GO_MODULE` comes from the `module` directive in `go.mod`, read from the
  host upstream actually develops on.** Mirrors serve stale module paths and the
  wrong value is not hypothetical: `gitea.com/gitea/tea` declares `gitea.dev/tea`
  while the GitHub mirror `go-gitea/tea` still declares `code.gitea.io/tea` on
  `master` — and the mirror is where a first look lands.
- **Do not package or install Go libs or source**, only *"the programs and
  things users need, not the things Go developers would need."*
- Dependency sourcing, in the handbook's order of preference: vendored deps,
  then the versions upstream specifies in `go.mod`, then versions current at
  release.
- A single-binary Go port needs **no `pkg-plist`** — `PLIST_FILES=` covers it.
  No Go port exists in this overlay yet, so there is no local Go precedent; the
  `PLIST_FILES=` *pattern* is in use by `sysutils/dtpstree` (which pairs it
  with `GNU_CONFIGURE`) and `sysutils/hardlink` (which pairs it with a
  hand-written `do-install`) — neither of which is Go. Two more ports,
  `devel/rubygem-jeweler` and `www/rubygem-adsf`, declare `PLIST_FILES=` for a
  `bin/` entry alongside `USES=gem`, so in-repo precedent for the pattern is
  four ports; only the first two rely on it alone.
- `GO_TESTTARGET` defaults to `./...`, so `make test` works with no extra work.
- **Completion files are always installed, with no option guarding them** —
  *"when available, completion files should always be installed. It is not
  necessary to make an option for it."* If an option is used anyway, always
  enable it in `OPTIONS_DEFAULT`. The handbook tabulates three shells:
  `${PREFIX}/etc/bash_completion.d` or
  `${PREFIX}/share/bash-completion/completions` for bash,
  `${PREFIX}/share/fish/completions/${PORTNAME}.fish` for fish, and
  `${PREFIX}/share/zsh/site-functions/_${PORTNAME}` for zsh. No other shell has
  a prescribed location. And *"do not register any dependencies on the shells
  themselves."*

## Step 5: Patching, if the software needs it

Prefer not to. `BINARY_ALIAS` substitutes hardcoded commands during the build
and avoids patching build files at all.

When a patch is needed, make the change against a `.orig` copy and let the
framework name the file:

```sh
cp file file.orig
# edit file
bundle exec rake makepatch
```

`rake makepatch` runs `make makepatch` with `PORTSDIR` pointed at the `githead`
tree, so `smart_makepatch.sh` — which decides patch naming and whitespace
handling — comes from the tree `update_githead` keeps current rather than from
`/usr/ports`. That prerequisite means the first run of a day needs Douglas's
sudo, like `rake portlint` and `rake diff`; bare `make makepatch` in the port
directory works too and needs none.

**Do not confuse this with `rake diff[patch]`**, which produces the single
`<PKGNAME>.diff` a PR attaches. These patches ship inside the port.

The naming rules it implements, from `slow-porting`: patches live in `files/`,
are relative to `WRKSRC` (`make -V WRKSRC`), replace each `_` with `__` and each
`/` with `_`, use only `[-+._a-zA-Z0-9]`, and never look like `patch-aa`. Avoid
two patches touching the same file. A patch may span several files if the change
is one thing and the name says so, as in `patch-add-missing-stdlib.h`.

Two specific rules worth keeping in mind:

- **To delete a file, use `post-extract`, not a patch.**
- **Never diff a generated `configure`.** If `configure.ac` must change, set
  `USES=autoreconf` and patch the `.ac` — `www/mod_wsgi` does exactly this with
  `files/patch-configure.ac`.

Keep non-functional whitespace out of patches: it inflates the tree and hides
what actually changed.

## Step 6: The packing list

Static lists are preferred — a static list is one a user can `grep` without
downloading and compiling the distfile, which includes a `pkg-plist` generated
by `make makeplist` before commit. Dynamic lists are for ports whose file list
changes drastically with options.

```sh
make makeplist
```

**Double-check the output** — the chapter is explicit that *"the output of
`makeplist` must be double checked for correctness as it tries to automatically
guess a few things, and can get it wrong."* Propose it; do not accept it.

Most ports here need no `pkg-plist` at all: 14 of the 22 generate it through
`USE_PYTHON=autoplist` or `USES=gem`, and two declare `PLIST_FILES=` inline. Check
which case you are in before writing a file.

Specifics:

- **Configuration files are never listed in `pkg-plist`.** Listing them makes
  `pkg delete` destroy files the user has edited. Install a `filename.sample`
  and use `@sample etc/foo.conf.sample`. For a non-standard suffix or a
  different source directory, the form is
  `@sample <sample-file> <actual-config-file>`.
- Config in a subdirectory of `${PREFIX}/etc` uses `ETCDIR`, which defaults to
  `${PREFIX}/etc/${PORTNAME}`, and `%%ETCDIR%%` in the plist.
- `info/dir` must not be listed; add the appropriate `install-info` lines.
- Shared libraries follow `special`'s shared-libraries section — no port here
  currently installs one.

## Step 7: pkg-descr and pkg-message

`pkg-descr` is the long description. `WWW=` goes in the **Makefile**, not in
`pkg-descr` — that moved years ago in a tree-wide sweep.

`pkg-message` only if something is *vital* to setup on FreeBSD and unique to
this port. Its rules:

- Setup information shows on **initial install** only; upgrade instructions only
  when upgrading from the relevant version. The UCL format (the file starts with
  `[`) expresses that with `type` and `maximum_version`; a plain-text file shows
  on install only.
- **No decorative framing** — no whitespace padding, no lines of `-----` or
  `=====`. Leave formatting to `pkg`.
- Refer to the proper tools: `service name start`, not
  `/usr/local/etc/rc.d/name start`; `sysrc name_enable=YES` to change
  `rc.conf`.
- **Do not list `pkg-message` in `pkg-plist`.**

## Step 8: The category Makefile

**The submission must add the port to its category `Makefile`, and this repo has
none.** `www/Makefile` does not exist here; the tree's carries 2,804
`SUBDIR +=` lines in `githead` (dated 2026-09-29) and 2,816 in `/usr/ports` —
two trees, which is why the count differs with where you look.

No `SUBDIR +=` instruction to a submitter appears anywhere in the 18 chapters —
the only place editing a category `Makefile` is prescribed is "Proposing a New
Category", a different and rare case. The evidence is nonetheless consistent:
`quick-porting` rejects `shar` because it *"does not add the relevant entry in
the category's Makefile"*, which only follows if the entry is expected in the
submission, and new ports from outside submitters carry it in the same commit —
`net/libmodbus` (`6479175b5`) and `mail/tarpitd` (`5cc52bcf7`) both do.

`rake diff` adds that hunk itself for any port the tree does not have yet,
inserting the `SUBDIR +=` line into the tree's category `Makefile` in byte
order. Check that it is there, in the right place, when you read the diff.

## Step 9: Lint and test

```sh
bundle exec rake portlint     # -A here, the most thorough mode, since the port is new
bundle exec rake portclippy
bundle exec rake portfmt
```

`portlint -A` is *"the most thorough"* for a new port. Its false positives are
documented — *"since portlint uses heuristics… it can produce false positive
warnings"* — the standing example being "consider adding support for a NLS
knob", which is wrong wherever the software cannot disable NLS at all. No port
here triggers it today, but a new port with `USES=…gettext…` and no NLS option
will.

**`rake testport` is Douglas's to run** (it needs sudo). Hand him the command.
For a new port it is the step that proves `pkg-plist`, staging and `stage-qa`,
so it is not optional. Where options alter the plist, the chapter asks for a run
with all options selected **and** one with all deselected (`-c` presents the
dialog).

**When a build runs long, read poudriere's log for the package that is
building** — whether it is `rake testport` or caronade (step 11), and whether
that package is the port or one of its dependencies. A background loop says
nothing until the end. caronade's job log carries the port's own build as it
happens, but for a dependency only a `Building <origin> | <pkgname>` line and
later a `Finished` line; that `<pkgname>` names the log to read. `rake testport`
shows you no output at all, so read the most recently modified log. Where
poudriere's logs are readable, each package's own log is
`/usr/local/poudriere/data/logs/bulk/<jail>-<tree>/latest/logs/<pkgname>.log`.
For `rake testport`, `<tree>` is `githead`; for caronade, the `Logs:` line near
the top of its job log names the run directory, and the package logs are in
its `logs/` subdirectory. Every package poudriere builds has `<phase: …>`
markers saying which phase its build is in (`build`, `stage`, `package`; the
port under test adds `install` and `deinstall`), and the log's modification time says
whether it is still writing. Once in `package`, the log writes nothing more
until it is done, so a long quiet spell there is normal.

To estimate how long a dependency will take — before setting any limit on a
background loop — look up its `build time:` line in earlier logs from the same
jail, `/usr/local/poudriere/data/logs/bulk/<jail>-*/2*/logs/<portname>-[0-9]*.log`;
the name carries the version, so the glob also finds builds of earlier ones.
Skip any log ending in `!!! build failure encountered !!!`: a failed run's
time is how long it took to fail.

Once `testport` passes, write the diff for review into the session scratchpad.
`rake diff[patch]` is for the attachment, after the commit (step 11), and
refuses to run before it:

```sh
bundle exec rake diff > <scratchpad>/<portname>.diff
```

## Step 10: Independent review

Once `testport` passes, and before anything goes to Bugzilla, dispatch a fresh
reviewer with the `Agent` tool (`general-purpose`). You wrote the port; every
check so far either ran a tool or ran past your own judgement. A reviewer with
none of this session's context reads it the way a committer will.

**Brief it with artifacts, not conclusions.** Give it the diff's path, the port
directory, the upstream
repository and release, and the Porter's Handbook at
<https://docs.freebsd.org/en/books/porters-handbook/>. Do **not** give it your
reasoning for choices — why this category, why no `USES=cpe`, why no
completions. A justification in the brief is the reviewer adopting your blind
spot instead of finding it; a choice that is right will survive being asked
about.

Ask it to:

- read-only: no edits, no `rake` targets that need `sudo`, no Bugzilla writes;
- check the diff against the handbook and against upstream's own sources
  (`go.mod`/`Cargo.toml`, license file, release tag), citing the chapter or
  file for every finding;
- report what it checked and found sound as well as what it found wrong, so
  silence on an area means "not examined", never "fine".

**Its findings are claims, not verdicts.** Verify each against the source it
cites before acting, and put the verified list in front of Douglas with what
you propose for each. A change that touches the port reruns step 9 —
`testport` included — **and then this step**, with a fresh reviewer and a
freshly written diff, before submission. A rename, a moved `SUBDIR` line or a
rewritten `pkg-descr` is exactly the kind of choice the earlier reviewer never
saw; passing lint and `testport` says nothing about whether it is right.

## Step 11: Submit

**Commit first, then submit.** Once Douglas has seen the verified review and
the port is final, commit it here — the attachment carries the commit's
author, date and message. Generate it with `rake diff[patch]`, which writes
`<PKGNAME>.diff` at the repo root in the form `git format-patch` gives, with
the category `Makefile` hunk included, so a committer can `git am` it. It
refuses to run while the port has uncommitted changes.

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
`target_url`. Watch with a background loop until no queue is `waiting` or
`building`. Do not draft the report until every queue has passed. A failure is
a change like any other — fix it, and the port goes back through steps 9 and
10.

A build that runs long is followed from poudriere's log for the package that
is building, as in step 9.

Submission title: `[NEW PORT] category/portname short description of the port`.
Product "Ports & Packages", Component "Individual Port(s)". From the Problem
Reports article: attachment not inline, MIME type `text/plain`, never
`quoted-printable`, `git format-patch` preferred so a committer can `git am` and
credit correctly, severity honest, and no compression.

**The description is short.** The handbook's "Submitting the New Port" asks
for *"a short description of the program"*, perhaps a short version of
`COMMENT`, and nothing else. Do not list the added files: that request is the
`upgrading` chapter's, for updates, and its reason — git needs them named at
commit time — does not survive `git am`, which adds them itself, while the
patch's own diffstat already names them. Do not say who will maintain it;
`MAINTAINER` says so. Douglas's own new-port PRs (bugs 217908, 217910, 217912)
take this form:

```
Added dns/rubygem-simpleidn [1].

"SimpleIDN allows easy conversion from punycode ACE (IDNA) strings to
unicode UTF-8 strings and visa versa."

[1] https://github.com/mmriis/simpleidn

portlint: OK (looks fine.)
testport: OK (poudriere: 103i386, 103amd64)
```

The quoted line is upstream's own description, and the testport line names the
jails it ran on. Their attachment descriptions were the patch's file name.

```sh
bugzilla attach --type text/plain --field is_patch=1 \
  --description '<summary>' --file <file> <bug>
```

Pass `--description`: without it the description silently defaults to the
file's basename, which is not the text you had approved.

## Messages sent in Douglas's name

Every Bugzilla comment, bug description and attachment description that goes out
as him is **approved or reshaped first, and the gate is not yes/no.** Show the
draft in full as it will render; use `AskUserQuestion` with options that offer
reshaping rather than send/hold; treat the free-text answer as the edit channel;
repeat until he approves.

Comments render inside `<pre class="bz_comment_text">`: **no Markdown**,
whitespace literal, so **hard-wrap it** — ~76 columns by convention — and
references by the `[N]` convention with a trailer block.

## Commits in this repo

A new port means new files, so `git add -N` each one as it is created — the repo
is otherwise easy to leave with untracked work. Commits here carry no Claude
trailers — see this repo's `CLAUDE.md`.

## Common Mistakes

- **Renaming the software because `PKGBASE` is taken.** The remedy is a prefix
  or suffix, never a changed `PORTNAME`.
- **Accepting `make makeplist` output unread.** The chapter says to double-check
  it; it guesses.
- **Listing configuration files in `pkg-plist`.** `pkg delete` then destroys
  user-edited files. Use `.sample` plus `@sample`.
- **Listing `pkg-message` in `pkg-plist`.** It does not go there.
- **Diffing a generated `configure`.** Set `USES=autoreconf` and patch
  `configure.ac`.
- **Briefing the reviewer with your reasoning.** Step 10's reviewer gets the
  diff and the sources, not why you chose what you chose — or it reviews your
  argument instead of the port.
- **Attaching `git format-patch` output.** The commit cannot carry the category
  `Makefile` hunk, since that file is not in this repo; `rake diff[patch]` adds
  it.
- **Reading `go.mod` from a mirror.** Read it from the host upstream develops
  on.
- **Taking `porting-samplem` as a template.** Read it for the callouts; its
  sample is dated.
- **Building a conclusion from output sliced with `head`/`tail`.** And an empty
  `grep` cannot distinguish "absent" from "the command died" — check the exit
  status alongside filtered output.
