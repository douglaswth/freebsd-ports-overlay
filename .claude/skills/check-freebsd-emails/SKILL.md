---
name: check-freebsd-emails
description: Classify FreeBSD ports mail by what each kind means and route it to the port skill that acts on it. Use when FreeBSD mail needs triaging, when the user asks what ports work is waiting on them, or when a mail-triage skill delegates that mail to this repo. Carries no mail transport of its own.
allowed-tools: Bash(bugzilla query *) Bash(gh api *) Bash(fetch -qo - *) Bash(git *) Bash(python3 *) Read Grep Glob AskUserQuestion
---

# Check FreeBSD Emails

FreeBSD ports mail is not a notification sidecar to the practice; it is where
the work originates. Every port operation except adding a new port has a mail
trigger, and the mail carries the arguments the operation needs.

**This skill owns the FreeBSD semantics, not the mail transport.** It says what
each kind of mail means and which port skill acts on it. Reading the mailbox,
marking messages read and moving them belong to whatever mail tooling the
session already has — a mail-triage skill that delegated this folder, or the
user's own helper. Follow that tooling's conventions for enumeration and for
every mutation; this file deliberately names none, so it works the same whether
it was invoked directly or read by a mail skill mid-triage.

Two consequences worth stating:

- **Enumerate once, in whatever form the mail tooling prescribes as its first
  call.** Those conventions usually make the fuller form a *substitute* for the
  bare one, not a follow-up, so adding flags on a second trip is wrong.
- **Mutations are the mail tooling's to run**, under its own confirmation rules.
  This file only says *which* messages qualify, and why.

## Prerequisite: is this repo operable?

`update-port`, `add-port` and `mirror-upstream-commit` all **edit and commit
here**. So before routing anything to them, `git fetch` and attempt
`git pull --ff-only`. Let git arbitrate; do not pre-check dirtiness, since a
dirty worktree does not block a fast-forward unless the incoming changes overlap
the modified files.

| Outcome | Action |
| --- | --- |
| Pull succeeded | Route normally |
| Refused, dirty | **Ask**: stash/pull/run/pop, commit first, triage only, or hold |
| Refused, diverged | **Ask**: `git pull --rebase`, triage only, or hold |
| Fetch failed | **Ask**: retry, proceed local-unverified if clean and undiverged, triage only, or hold |

Classifying the mail is safe either way — it is the routing into a skill that
edits which needs the repo sane. Say which mode you are in rather than silently
doing less.

## Step 1: Classify

By FreeBSD semantics, not by generic triage. The subject line is usually enough
to classify; the body carries the arguments.

| Mail | Carries | Routes to |
| --- | --- | --- |
| portscout "ports you maintain which are out of date" | port, current version, new version | `update-port` |
| `maintainer-feedback requested:` / `maintainer-approval requested:` | bug number, attachment id | `review-port-pr` |
| commit-hook "A commit references this bug" | upstream commit sha | `mirror-upstream-commit` |
| FreshPorts watch-list notification | a commit to a maintained port, by anyone | `mirror-upstream-commit` |
| pkg-fallout failure | jail, architecture, failing phase, log URL | `update-port`, or a fix |
| "Problem reports … need special attention" | the open PRs awaiting action | reconcile, step 2 |
| Bug mail where the maintainer is only on CC | nothing actionable | no action |

Three classifications that are easy to get wrong, each found by getting it
wrong:

- **Read the requestee, not the flag name.** A `maintainer-feedback?` in a
  subject does not mean it is addressed to the maintainer. Bugs arrive because
  he is CC'd, and their flags are often requested of other people. Check who the
  flag is set on before routing to `review-port-pr`.
- **A flag on a closed bug is residue, not work.** Bug 298985 is Closed FIXED
  with `maintainer-feedback?` still set on him. Nothing is owed. Do not present
  these as a queue.
- **Human correspondence from committers is real queue mail and may not be
  filed with the rest.** Service senders (bugzilla, portscout, pkg-fallout,
  FreshPorts) are easy to filter on; mail from an individual committer — often
  the one who handles these PRs — is not, so it can land wherever unfiled mail
  goes. If the user mentions a thread that is not among the FreeBSD mail you
  enumerated, that is the likely reason; say so rather than concluding it does
  not exist.

## Step 2: Reconcile the digest against live Bugzilla

The "need special attention" digest is a **snapshot**, and its only action is
reconciliation. Re-derive the state rather than trusting it:

```sh
bugzilla query --product "Ports & Packages" --cc <maintainer-email> --status New
bugzilla query --product "Ports & Packages" -f 'maintainer-feedback?' --cc <maintainer-email>
bugzilla query -b <bug> --oneline
```

**Query `--cc`, not `--assigned_to`.** A ports bug is assigned to
`ports-bugs@FreeBSD.org` or to whichever committer picks it up — never to the
maintainer, who is CC'd instead. Measured: `--assigned_to` returns **1** bug
while `--cc` returns **144**, including every item this skill discusses. Using
the assignee field makes the step report an empty queue, which looks like
"nothing is rotting".

Two smaller flag notes. `--assigned-to` with a hyphen is not an alias of
`--assigned_to` and exits 2 with `unrecognized arguments`. And **`--oneline` is
the format that shows flags**; `--full` does not print them, which matters here
because residual flags are this step's whole subject.

Two properties of the digest worth knowing:

- It enumerates **open** problem reports by status, so a resolved bug cannot
  appear in it however its flags are set. That is correct behaviour, not a gap.
- It is sent **weekly**, so a bug that opens and resolves inside a week never
  appears in one at all. Bug 298985 did exactly that — filed 30 September,
  committed 3 October, never digested. So the digest is a backstop for things
  rotting, never a complete queue.

`bugzilla query` does not expose Bugzilla's web filters one-for-one —
`--emailcc1` is not a flag — but `--cc`, `--field FIELD=VALUE` and `--from-url`
(paste an Advanced Search URL) cover it. Reach for those before hand-rolling
REST. The one thing the CLI will not give you is the comment thread; that comes
from `https://bugs.freebsd.org/bugzilla/rest/bug/<bug>/comment`.

## Step 3: pkg-fallout

Its mail-side job is parsing the body: which jail, which architecture, which
phase failed, and the log URL. Fetch the log before concluding anything — the
failing phase names the kind of problem (`configure`, `build`, `stage-qa`,
`package`), and a `stage-qa` or plist failure is a different fix from a compile
error.

The port-side outcome is just `update-port` or a direct fix, which is why this
has no skill of its own.

## Step 4: Route, and continue in the same session

The four doing-skills live beside this one:

```
.claude/skills/review-port-pr/SKILL.md
.claude/skills/update-port/SKILL.md
.claude/skills/mirror-upstream-commit/SKILL.md
.claude/skills/add-port/SKILL.md
```

Invoke them by name when this repo is the session's working directory. When it
is not, they are project-scoped and invisible to the `Skill` tool, so read the
file by path instead — and read it **before** issuing any call on its behalf,
since each prescribes its own first invocation and a wrong first call is not
repaired by a second.

Keep going rather than handing back. When several items route to the same skill,
say so and let the user pick the order — a security-motivated update outranks a
routine version bump, and a `maintainer-feedback?` someone is actively waiting
on outranks both.

## Step 5: Marking read

This is the one mail mutation with a FreeBSD-specific rule, which is why it is
here rather than left entirely to the mail tooling.

**portscout's mail is a notice, not the queue — the queue is the maintainer's
portscout page.** The mail says it will not repeat itself (*"You will not be
e-mailed again for any of the port/version combinations below"*), but portscout
keeps every outdated port listed at
`https://portscout.freebsd.org/<maintainer-email>.html` with its current and new
version until the port is updated, and it never sees the mailbox, so a message's
read state changes nothing on its side. When portscout mail arrives, read that
page, not only the mail: it carries ports the mail did not mention, and it is
what tells you whether a bump is still owed. Whether to keep a portscout message
unread as a personal reminder is the maintainer's choice, not an obligation.

The rest divides cleanly:

- **Safe to mark read**: residue (flags on closed bugs), CC'd-for-information
  mail, anything whose work is finished, and portscout mail once its ports are on
  the page.
- **Leave unread**: any `maintainer-feedback?`/`maintainer-approval?` genuinely
  requested of the maintainer and not yet answered.

Hand the mail tooling the exact set you enumerated and summarised — never a
range or a predicate — and let its own confirmation gate run.

## Common Mistakes

- **Presenting flags on closed bugs as a queue.** They are residue; the digest
  is right to ignore them.
- **Assuming a `maintainer-feedback?` in a subject is addressed to the
  maintainer.** Read the requestee.
- **Treating the weekly digest as the queue.** It misses anything that opens and
  closes inside a week, and it only ever lists open reports.
- **Treating the portscout mail as the queue.** The maintainer page lists every
  outdated port, including ones no mail mentioned; the mail is only the first
  notice.
- **Concluding from `bugzilla query --full` that a bug has no discussion.** It
  prints no comments at all — use the REST comment endpoint.
- **Concluding a thread does not exist because it is not in the FreeBSD mail.**
  Individual committers' mail may be filed elsewhere entirely.
- **Routing into a skill that edits before checking the repo pulls.** Those
  skills commit; a repo mid-work will bundle changes they did not make.
- **Slicing the enumeration with `head`/`tail` and classifying what survived.**
  The slice is chosen before you know where the answer is; it cost two handbook
  chapters and a poudriere tree elsewhere in this project. Read it whole or
  filter by name, and check exit status alongside any filtered output.
