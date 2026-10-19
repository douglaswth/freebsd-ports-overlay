---
name: add-port
description: Create a new FreeBSD port from scratch in this overlay, test it, and submit it to Bugzilla as a new-port PR. Use when the user wants to package software that has no port yet.
allowed-tools: Bash(bugzilla query *) Bash(bugzilla attach *) Bash(bugzilla new *) Bash(bugzilla modify *) Bash(portedit *) Bash(gh api *) Bash(fetch -qo - *) Bash(bundle exec rake *) Bash(make *) Bash(cp *) Bash(git *) Bash(grep *) Read Edit Write Grep Glob WebFetch WebSearch AskUserQuestion
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

`rake diff` compares a port **directory**, so it can never generate that hunk.
Construct it against the real tree: read the category `Makefile` from upstream,
insert the `SUBDIR +=` line in sorted position, and produce that hunk by hand as
part of the patch.

## Step 9: Lint, test, submit

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

Then:

```sh
bundle exec rake 'diff[patch]'
```

Submission title: `[NEW PORT] category/portname short description of the port`.
Product "Ports & Packages", Component "Individual Port(s)". From the Problem
Reports article: attachment not inline, MIME type `text/plain`, never
`quoted-printable`, `git format-patch` preferred so a committer can `git am` and
credit correctly, severity honest, **added files named explicitly** since git
needs them named at commit time, and no compression.

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
- **Forgetting the category `Makefile` hunk.** `rake diff` cannot produce it, so
  nothing will remind you.
- **Reading `go.mod` from a mirror.** Read it from the host upstream develops
  on.
- **Taking `porting-samplem` as a template.** Read it for the callouts; its
  sample is dated.
- **Building a conclusion from output sliced with `head`/`tail`.** And an empty
  `grep` cannot distinguish "absent" from "the command died" — check the exit
  status alongside filtered output.
