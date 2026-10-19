---
name: review-port-pr
description: Verify someone else's patch against a FreeBSD port Douglas maintains, then report and optionally answer on Bugzilla. Use when a maintainer-feedback or maintainer-approval request arrives, or when the user names a PR number for one of these ports.
allowed-tools: Bash(bugzilla query *) Bash(bugzilla attach --get *) Bash(bugzilla modify *) Bash(gh api *) Bash(fetch -qo - *) Bash(bundle exec rake *) Bash(make -V *) Bash(git *) Bash(sha256 *) Bash(sha512 *) Bash(md5 *) Bash(sha1 *) Bash(base64 *) Bash(python3 *) Read Grep Glob WebFetch WebSearch AskUserQuestion
---

# Review a Port PR

Verify a patch somebody else submitted against a port in this overlay. The verb
is **verifying a claim**, not making a change — if the job is to bump a port
yourself, that is `update-port`.

The handbook's reason this role exists, from `security`: *"being a ports
committer is not enough to commit to an arbitrary port. Remember that ports
usually have maintainers, who must be respected."* A committer is waiting on an
answer, so the review ends in one.

## When to Use

- A `maintainer-feedback requested:` or `maintainer-approval requested:` mail
  arrives.
- The user names a bug number, or asks about a PR against one of these ports.
- A commit-hook mail cites a bug whose patch was never reviewed.

## Step 1: Read the bug

```sh
bugzilla query -b <bug> --full
```

**That does not print the comments.** What it prints, verified against bug
299391, is id, status, assignee, summary, component, CC, Blocked and Depends —
**no product and no flags**. For flags use `--oneline`, which renders them:
`#299391 Closed … [---] maintainer-feedback+`. The comment thread — which is where the submitter's reasoning,
the flag requests and any committer discussion live — comes from the REST API:

```sh
fetch -qo - 'https://bugs.freebsd.org/bugzilla/rest/bug/<bug>/comment'
```

Pipe it through `python3 -c` to print `count`, `creator`, `creation_time` and
`text` per comment. Read **every** comment, oldest first: the patch you were
asked about may already have been superseded in a later comment, or the
submitter may have answered a question you are about to ask.

Note who the flag is requested **of**. A `maintainer-feedback?` in a mail
subject does not mean it is addressed to Douglas — he is on the CC list for
bugs that have nothing to do with him, and those flags are requested of other
people.

## Step 2: Get the patch

```sh
bugzilla attach --get <attachment-id>
```

Attachment ids come from the comment thread (`Created attachment 275690`). Take
the **newest** patch unless the thread says otherwise, and check whether earlier
ones are marked obsolete.

## Step 3: Read the upstream release notes

**Do this before examining the patch, every time, for any version bump.**

Nothing in a patch says "security". PR 299391 looked like routine packaging —
`Makefile`, `distinfo` and a shrunk `files/patch-configure.ac`, with
`PORTREVISION` dropped — and its release notes disclosed GHSA-q67r-wxfm-97hj, a
race at process start that leaked request context between requests. Verifying the
mechanics of a patch cannot find that; only reading what the release contains
can.

Use `WebFetch` on the project's release notes and, where one exists, its
security advisories page. What you are looking for:

- A security fix. If there is one, the review gains two deliverables: the commit
  message needs `Security: <advisory id>`, and **a VuXML entry is required** —
  `pkg audit` reads VuXML and nothing else. Ask the submitter for it if they are
  a committer; it is theirs to add with the port commit.
- Behaviour changes that belong in `UPDATING`.
- Requirement floors that moved (see step 6).
- Whether a fix in this release makes an existing `files/patch-*` redundant.

## Step 4: Verify the distfile

Recompute rather than trust. Fetch the distfile from the master site and compare
the hash and size against the `distinfo` the patch writes:

```sh
make -V DISTNAME -V DISTFILES -V MASTER_SITES
fetch -qo - '<master-site><distfile>' | sha256
```

`rake md5` / `sha1` / `sha256` / `sha512` print sums of already-fetched
distfiles, for checking against the hash the **project publishes** — PyPI,
GitHub releases and most projects publish one. `CHECKSUM_ALGORITHMS` is `sha256`
alone, so that is the only one `distinfo` carries.

A `TIMESTAMP` that does not change when `distinfo` does is not a defect by
itself — upstream's own copies lack it on older ports and it is rewritten
whenever `makesum` next runs.

## Step 5: Confirm the patch applies to current `main`

Two routes. The cheap one uses the local `githead` tree, which is a real git
checkout but root-owned, so git needs an explicit exception:

```sh
git -c safe.directory=/usr/local/poudriere/ports/githead \
    -C /usr/local/poudriere/ports/githead apply --check <patch>
```

`apply --check` writes nothing. **But check the tree's vintage first** —
`git -c safe.directory=… -C … log -1 --date=short --pretty='%h %ad'`. `rake`
refreshes `githead` only on the first task of a day that depends on
`update_githead`, so a day spent on `portclippy`/`portfmt` (which carry no such
prerequisite) leaves it weeks behind. If it predates the patch, either ask
Douglas to run `rake update_githead[force]` — it needs his sudo — or use the
second route.

The second route reads upstream directly and needs no tree:

```sh
gh api "repos/freebsd/freebsd-ports/contents/<category>/<port>/Makefile?ref=main" \
  --jq '.content' | base64 -d
```

Compare that against the patch's context lines. This is also how you answer
"has someone already committed this?" — which is worth asking before spending
the review.

## Step 6: The checklist

Work through all of it. Each item has produced a real defect.

- **For any dependency the patch adds or keeps, ask whether it should exist at
  all** — not merely whether it is genuine. The two questions come apart. The
  6.1.0 review got this exactly wrong: it confirmed `gettext-runtime` was a real
  dependency (Python's `sysconfig` `LIBS` carries `-lintl`, and the built
  module's `NEEDED` listed `libintl.so.8`) and passed the patch on that basis.
  6.1.1's notes then explained that appending Python's `LIBS`/`SYSLIBS` was
  itself the bug, masking a missing `-L${PYTHONLIBDIR}`. The `NEEDED` entry
  cited as proof was the symptom of the defect, not evidence against it.
- **`PORTREVISION`**, by the `security` chapter's rule: bump it *"if adding a
  patch file to the port, but do not bump it if updating the port to the latest
  software version and thus already touched `DISTVERSION`"*. A packaging-only
  change must bump it; a version bump must not also.
- **Any `USES` change is a lookup against the `uses` chapter**, never a
  judgement. `USES=gettext` is deprecated there in favour of `gettext-runtime` /
  `gettext-tools`, and which replacement is right depends on whether the build
  runs `msgfmt`.
- **A requirement floor that moved.** Both `USES=python` and `USES=apache` take
  version arguments (`python:3.11+`, `apache:2.4+`) and that is the documented
  way to declare a minimum. A moot constraint costs nothing; an undeclared one
  silently stops being moot. Resolve which versions exist with
  `make -V FLAVORS` and `_PYTHON_VERSIONS` in the tree you name — do not reason
  about it.
- **`CPE_STR` re-verified against the NVD dictionary** whenever a version moves
  or a CPE field changes. `USES=cpe` carries a prohibition: *"do not ever make
  up CPE data"*. Find the official entry, compare `make -V CPE_STR` to it, and
  override `CPE_PRODUCT`, `CPE_VENDOR`, `CPE_VERSION` or `CPE_UPDATE` one field
  at a time.
- **Whether `pkg-plist` should have changed**, against what the framework
  generates rather than against the diff. Most ports here generate it
  (`USE_PYTHON=autoplist`, `USES=gem`); two declare `PLIST_FILES=` inline. And
  per the `testing` chapter, where options alter the plist the port wants a test
  run with all options selected **and** one with all deselected.
- **The `UPDATING` entry**, judged against the format `upgrading` specifies
  (`AFFECTS: users of portcategory/portname`, `AUTHOR:`) and against whether it
  names the breakage that will actually stop a service starting.
- **If the patch marks the port**, is it the right marker? `BROKEN` is
  temporary, `IGNORE` structural, and `FORBIDDEN` security only — *"do not mark
  a port `FORBIDDEN` for any reason other than security"*. Values are unquoted,
  `CONFLICTS` is what file conflicts use since it sets `IGNORE` itself, and
  **never `.error`**, which breaks `make describe` and therefore
  `/usr/ports/INDEX`.

## Step 7: Lint and test

`bundle exec rake portlint` (`-C` here, since the port exists),
`bundle exec rake portclippy`, `bundle exec rake portfmt`. A recent `portlint`
runs `make describe` itself, so that is covered.

`portlint` and `diff` depend on `update_githead`, which runs
`sudo poudriere ports -u` on the first task of any day — so the first of these
may want Douglas's sudo.

Expect `portclippy` and `portlint` to disagree on variable order. Run both,
reconcile by hand, treat neither as authoritative — and do **not** expect
`rake portfmt` to fix a `portclippy` complaint. It aligns tabs and sorts tokens
within a variable; it does not reorder variables.

**`rake testport` is Douglas's to run** — it needs sudo, which fails in a Claude
session. Hand him the command rather than gating on it:

```sh
rake testport
```

Claude cannot test uncommitted work at all. The caronade route only applies to
pushed commits, so it has no part in reviewing an attachment.

## Step 8: Report in chat first

Give the verdict and the evidence to the user before touching Bugzilla.
Structure it as: what the patch claims, what you verified, what you found, and
what you recommend (approve / request changes / needs more from the submitter).
Say which checklist items you could not settle and why.

## Step 9: Answer on Bugzilla, if the user wants one

Only after they have seen the review. Two writes are available:

```sh
bugzilla modify -l '<comment>' --flag maintainer-approval+ <bug>
bugzilla modify -l '<comment>' --flag maintainer-feedback+ <bug>
```

`--flag <name>+` sets, `--flag <name>?` requests, `--flag <name>X` clears.

**Every message that goes out in Douglas's name is approved or reshaped first,
and the gate is not yes/no.** Show the draft **in full**, exactly as it will
render. Use `AskUserQuestion` with options that offer reshaping, not merely
send/hold, and treat the free-text answer as the edit channel. Repeat until he
approves. A summary of a draft is not a draft — the text that goes out is the
text he has read.

Writing the comment itself:

- **There is no Markdown.** Comments render inside `<pre class="bz_comment_text">`,
  so whitespace is literal and **hard-wrapping is necessary** — an unwrapped
  paragraph forces horizontal scrolling for every reader. ~76 columns is the
  convention FreeBSD's own comments follow; the exact figure is a convention,
  the need to wrap is not.
- What *is* linkified, from `quoteUrls` in `Bugzilla/Template.pm`: bare URLs
  and full `show_bug.cgi?id=N#cM` links; `bug #N`, `comment #N`,
  `bug #N, comment #M`, and the plural forms `bugs 1, #2, 3` and
  `comments 1, 2`; `attachment #N` optionally followed by `[details]`; bare
  email addresses; lines beginning with `>`; and the duplicate marker. FreeBSD's
  own help panel adds `review D123` for Phabricator. The SVN forms (`ports rN`)
  died with the 2021 git migration, so a git hash needs a full cgit URL.
- **For references use the `[N]` convention** — a bare `[1]` inline, with the
  matching `[1]` in a trailer block at the end. That is FreeBSD's idiom and
  needs no markup, since the reference is usually a URL or a `bug #N` that
  linkifies by itself.

If the patch needs changes, the comment says what and why, with the evidence —
a release-note quote, a hash, a `Mk/` line — not just a verdict.

## Common Mistakes

- **Concluding from `bugzilla query --full` that a bug has no discussion.** It
  does not print comments at all. Use the REST comment endpoint.
- **Confirming a dependency is real and stopping there.** Real and *correct* are
  different questions; see step 6.
- **Quoting an author date as a commit date.** They routinely differ by days on
  ports commits — `0d01e25f7` was authored 2026-09-30 and committed 2026-10-03.
  `gh api` returns both; say which you are using.
- **Treating `pkg audit` clean as evidence the installed package is safe.** It
  reports what the *local* VuXML copy knows, which can lag the committed entry
  by days, and no entry may exist at all. Check `/var/db/pkg/vuln.xml`'s date.
- **Reasoning about deployment from a remembered banner.** What is installed on
  a host is a different question from what the port says, and a `Server:` header
  read two weeks ago is not current. Re-read the installed package.
- **Reporting `portlint`'s "consider adding support for a NLS knob" as a
  finding.** It fires when `USES` matches `gettext` and the Makefile has neither
  an NLS option nor `NLS_USES`, and it is a false positive wherever the software
  cannot disable NLS at all — the handbook's precondition for an NLS option is
  that *the software* supports disabling it. No port here currently triggers it:
  `www/mod_wsgi` did until `d4319e0` dropped `gettext-runtime`, and
  `sysutils/psmisc` suppresses it with `NLS_USES= gettext`.
- **Using `git diff` in this repo to see what the patch changes relative to
  FreeBSD.** It compares against this repo's own history and never against what
  FreeBSD ships. `rake diff` answers that question.
- **Building a conclusion from output you sliced with `head`/`tail`.** The slice
  is chosen before you know where the answer is. Read it whole, or grep for the
  thing by name — and check the exit status alongside any filtered output, since
  an empty filter result cannot distinguish "absent" from "the command died".
