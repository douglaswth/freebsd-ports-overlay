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

## Rakefile

`rake` targets for working on ports. Note the Rakefile shells out to
`poudriere ports -l -q` at load time, so every target needs poudriere installed.

- `rake` / `rake update_githead` — create or update the `githead` poudriere ports tree
- `rake testport[jailname,flavor]` — copy the port's git-tracked files into `githead` and run `poudriere testport`
- `rake portlint` — `portlint -C` for existing ports, `-A` for new ones
- `rake makesum`, `rake distclean`, `rake md5` / `sha1` / `sha256`

## Testing before submitting

`portlint` (`-A` for a new port, `-C` for an update) and `poudriere testport`,
which also covers `pkg-plist` verification and `stage-qa`. The Porter's Handbook
"Testing" chapter is the current authority on what is expected.

Test one jail per live FreeBSD branch, at the newest point release of each. Do
not hardcode which versions or architectures those are — read the minimum
supported `OSVERSION` out of `Mk/bsd.port.mk` (it errors below a fixed value),
and take the architectures worth testing from the current Tier 1 set at
<https://www.freebsd.org/platforms/>.
