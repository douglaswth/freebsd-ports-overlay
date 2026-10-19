# FreeBSD Ports Overlay

Working copy of the FreeBSD ports maintained upstream by
`douglas@douglasthrift.net`. The authoritative copy of every port here is the
FreeBSD ports tree; this repo is where changes are prepared before they are
submitted, and where upstream changes are mirrored back.

## Commit messages

**Never add `Co-Authored-By: Claude ...` or `Claude-Session: ...` trailers to
commits in this repo.** Commits here follow FreeBSD ports conventions and their
content feeds patches submitted upstream via Bugzilla; a Claude trailer is both
off-convention and would leak into an upstream submission. This overrides any
default commit-message template.

The trailers that do belong are FreeBSD's, and only when they came from the
upstream commit being mirrored:

    PR:		123456
    Submitted by:	Some Contributor <contributor@example.com>
    Reviewed by:	Some Committer <committer@example.com>
    Approved by:	Douglas Thrift <douglas@douglasthrift.net> (maintainer)
    Approved by:	portmgr (blanket)

## Mirroring upstream commits

Upstream changes are brought in with `git am`, not by hand-editing, so the
original author and author date are preserved and only the committer becomes
Douglas. GitHub serves any ports commit as a ready-made mbox:

```sh
fetch -qo - https://github.com/freebsd/freebsd-ports/commit/<sha>.patch | git am
```

A patch that touches ports outside this overlay can be filtered — `git am`
passes `--include` and `--exclude` through to `git apply`:

```sh
fetch -qo - https://github.com/freebsd/freebsd-ports/commit/<sha>.patch |
    git am --include='dns/rubygem-simpleidn/*'
```

GitHub's `.patch` endpoint returns 403 intermittently. The same commit can
succeed and then be refused a minute later, and it does not track patch size or
file count, so it has to be handled rather than assumed away. The piped form
above fails safely (`git am` gets an empty stream), but any variant that writes
to a file must check the fetch succeeded, or it will apply whatever patch was
left there last. The fallback is to generate the patch from a local clone of
the ports tree:

```sh
git -C <ports-clone> format-patch -1 --stdout <sha> | git am
```

**A patch only applies if the local file matches the context it was authored
against.** Upstream tree-wide sweeps (removing `# $FreeBSD$`, moving `WWW=` into
Makefiles, `USE_RUBY=yes` → `USES=ruby`, man pages to `share/man`) change that
context, so patches must be replayed in upstream author-date order, or the port
directory must first be synced to the state the patch expects. `git am -3` does
not rescue a mismatch here: the blobs named in the patch's `index` lines are not
in this repo's object store unless the ports tree has been fetched as a remote.

Finding the commits that touch a given port:

```sh
gh api "repos/freebsd/freebsd-ports/commits?path=dns/rubygem-simpleidn&since=<iso8601>" \
  --jq '.[] | "\(.sha[0:9]) \(.commit.author.date[0:10]) \(.commit.message|split("\n")[0])"'
```

## Caronade CI

A Gitea push webhook on this repository drives caronade, which derives the
affected ports from each commit's **changed file paths** — the first two
segments of any added or modified path at least three deep, so a commit
touching `www/mod_wsgi/Makefile` generates a job for `www/mod_wsgi`. Commit
subjects are never consulted, whatever caronade's own README says. A commit
that touches only root-level files, this one included, generates nothing.

Jobs are generated per commit, on the queues listed under `default_queues`.
That setting lives in caronade's own config on the CI host, at
`/usr/local/etc/caronade/caronade.yaml` — root-owned, and generated from the
configuration-management repo, not a file in this repository. A line `CI: no` in the
commit message suppresses job generation; `CI: yes` forces all queues. With
neither, the default applies. The tag has to start its line, and is matched
case-insensitively with `no` tested before `yes`, so any `ci:` line containing
"no" anywhere counts as no.

Every mirrored upstream commit touches a port directory, so **mirroring in bulk
will match en masse** and queue a job per commit per default queue. Before any
bulk import, either disable the webhook for the duration or add `CI: no` to the
messages. Upstream commits have already been built by the FreeBSD cluster, so
there is nothing for CI to add.

## Rakefile

`rake` targets for working on ports. Note the Rakefile shells out to
`poudriere ports -l -q` at load time, so every target needs poudriere installed.

- `rake` / `rake update_githead` — create or update the `githead` poudriere ports tree
- `rake register_overlay` — register this working copy as a poudriere ports tree
  named `overlay`, via `poudriere ports -c -m null -M <repo> -f none`. `-m null`
  means poudriere registers the directory without creating or populating it, and
  `-f none` is required or the create errors on the directory already existing.
- `rake testport[jailname,flavor]` — `poudriere testport -b latest -p githead
  -O overlay`. `-O` takes a registered tree *name*, not a path, and poudriere
  nullfs-mounts it over the base tree, resolving ports through it first
  (`common.sh:874-875`, `:2310-2313`). So the build reads this working copy
  directly. `githead` stays as the base tree — it supplies `Mk/`, the
  dependencies and every port not carried here.

  `-b latest` prefetches binary packages for the dependencies rather than
  building them, matching the cron bulk builds configured outside this repo and
  caronade's own `poudriere.mk`. Without it a single port update rebuilds its
  whole dependency tree from source.

  This replaced copying the port's files into the root-owned `githead` with
  `sudo cp`, which is why `testport` now needs only one `sudo`, for the build
  itself. Removing that one would mean `poudriered`, and it is not worth it yet:
  `poudriere queue` is the client and takes `bulk` and `testport`, but the man
  page has said EXPERIMENTAL since 2018-03-08, and `write_usock` is `nc -U` with
  a heredoc that never reads a reply — so you get no build output, which is most
  of what `testport` is for.
- `rake diff[tree,patch]` — what this port changes relative to the ports tree,
  which is what a PR attaches. Defaults to `githead`; `rake diff[default]` uses
  `/usr/ports`. `rake diff[patch]` writes `<PKGNAME>.diff` at the repo root,
  which `.gitignore` covers.

  It uses `git diff --no-index` between the tree's copy of the port and this
  one, so nothing is copied into the ports tree and no `sudo` is involved. The
  paths are rewritten to the `a/`/`b/` form a submitted diff carries.

  **`git diff` in this repo does not answer the same question.** It compares
  against this repo's own history, so once a change is committed here it shows
  nothing, and it never shows what differs from what FreeBSD ships. The
  Porter's Handbook's `git diff --staged` assumes you are working inside a
  ports tree checkout; this repo is an overlay.
  `PKGNAME` is resolved against the tree being diffed, since it depends on
  `Mk/` — `PYTHON_PKGNAMEPREFIX` and friends come from there.
- `rake portlint` — `portlint -C` for existing ports, `-A` for new ones. Which
  flag it picks comes from whether the port's `Makefile` exists in `githead`;
  the lint itself runs against this working copy, nothing is copied anywhere.
- `rake portclippy` — `portclippy`, which checks variable **order** against the
  handbook's `order` chapter. Not redundant with `portlint`, and the two do not
  agree: `security/py-SecretStorage` is clean under `portclippy` while
  `portlint` wants `RUN_DEPENDS` earlier. Treat neither as authoritative over
  the other. 13 of the 22 ports here currently fail it.
- `rake portfmt` — `portfmt -i`, formatting the Makefile in place.
- `rake makesum`, `rake distclean`
- `rake makepatch` — regenerate `files/patch-*` from `.orig` copies under
  `WRKDIR`, via `make makepatch`.

  **Three different things share the word "patch" here.** `rake diff[patch]`
  writes the single `<PKGNAME>.diff` a PR attaches, patching the *ports tree*.
  `files/patch-*` ship inside the port and patch the *software*, and
  `make makepatch` is what generates them. They are unrelated artifacts.

  The underlying `make makepatch` **never fetches** — it regenerates patches
  from `.orig` copies already under `WRKDIR` — so the task needs no `DISTDIR`
  override, unlike `makesum`. `WRKDIR` and `PATCHDIR` resolve inside this
  working copy whichever tree `PORTSDIR` names, verified identical under both.
  The one thing it takes from the tree is `SCRIPTSDIR`'s `smart_makepatch.sh`,
  which decides patch naming and whitespace handling — hence `with_portsdir`,
  and hence the `rake` task carrying an `update_githead` prerequisite that the
  `make` target itself has none of. That is a uniformity fix rather than a
  correctness one: bare `make makepatch` in a port directory works, and the two
  trees' copies of the script were byte-identical when this was written.
- `rake md5` / `sha1` / `sha256` / `sha512` — sums of the fetched distfiles, for
  checking one against the hash **upstream publishes**. Not for `distinfo`:
  `CHECKSUM_ALGORITHMS` is `sha256` alone and `rake makesum` writes it. The set
  tracks what projects publish, which is why it spans four algorithms and not
  the dozen `/sbin` offers — nothing publishes `sha384`, `rmd160` or `skein*`.

`portlint` and `diff` set `PORTSDIR` to the `githead` tree for the commands they
run, via the `with_portsdir` helper. It matters because `PORTSDIR` otherwise
defaults to `/usr/ports`, which nothing here manages and which can be days
behind — and `portlint` reads `MOVED`, `UIDs`, `GIDs` and `Mk/Uses/*.mk` out of
it, so rename and removal checks were being made against a stale copy. The
helper sets and restores the variable around the call rather than prefixing the
command, so `rake` still echoes `portlint -C` and the override does not leak
into another task in the same invocation.

`portclippy` and `portfmt` need no ports tree at all — they parse the Makefile —
so unlike `portlint`, `diff` and `testport` they carry no `update_githead`
prerequisite and never need `sudo`.

## Testing before submitting

`portlint` (`-A` for a new port, `-C` for an update) and `poudriere testport`,
which also covers `pkg-plist` verification and `stage-qa`. The Porter's Handbook
"Testing" chapter is the current authority on what is expected.

Submission goes through Bugzilla, product "Ports & Packages", component
"Individual Port(s)", titled `category/portname: Update to X.Y`. The handbook
prefers `git format-patch` over a plain diff, because it carries author identity
and applies with `git am` — the same mechanism used here for mirroring upstream.
Generate it from the base of the tree, mention added or deleted files explicitly
since git needs them named at commit time, and do not compress it. Subversion is
not mentioned anywhere in the current handbook; `svn.freebsd.org` still serves a
read-only archive frozen at r569609 from the April 2021 git migration, which is
why the old svn-based `rake diff` was replaced rather than repaired — it would
have produced a confident diff against a five-year-old tree.

Test one jail per live FreeBSD branch, at the newest point release of each. Do
not hardcode which versions or architectures those are — read the minimum
supported `OSVERSION` out of `Mk/bsd.port.mk` (it errors below a fixed value),
and take the architectures worth testing from the current Tier 1 set at
<https://www.freebsd.org/platforms/>.
