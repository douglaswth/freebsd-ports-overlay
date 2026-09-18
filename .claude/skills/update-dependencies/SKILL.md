---
name: update-dependencies
description: Use when the user asks to check for or update gem dependencies, outdated gems, or security updates
allowed-tools: Bash(bundle *) Bash(git *) Bash(unlock-gpg *) AskUserQuestion
---

# Updating Dependencies

## Overview

Check for outdated Ruby gems and update them, investigating the nature of each update before proceeding.

## When to Use

- User asks to check for dependency updates
- User asks about outdated gems or security patches
- User asks to update gems

## Workflow

### 1. Check for outdated gems

```bash
bundle outdated
```

```bash
bundle outdated --strict
```

Run **both** bundler checks, for the same reason as the two uv checks: plain `bundle outdated` lists gems a newer version exists for *regardless of whether your constraints allow it*, while `--strict` lists only what could actually move. The difference is the set of gems capped by a parent, and you want to see both — plain tells you what is being blocked, `--strict` tells you what `bundle update` will really do. (There is no `bundle update --dry-run`; the flag does not exist.)

### 2. Investigate each update

For each outdated gem, determine whether the update is:
- **Security fix** (CVE patches)
- **Bug fix**
- **New feature / enhancement**
- **Data refresh** (e.g., mime-types-data, public_suffix)
- **Housekeeping** (gem size reduction, CI changes)

Check changelogs on GitHub or RubyGems for details. Present a summary table to the user with gem name, current version, latest version, and nature of the update.

**Read the changelog; never infer the update type from the version number.** A patch bump routinely carries a security fix. Observed: `json 2.21.1 → 2.21.2` was a use-after-free, `graphql 2.6.5 → 2.6.9` carried two advisories, `sqlparse 0.5.5 → 0.6.0` carried four DoS CVEs. Version shape is not evidence of anything.

**Diff from the version installed *here*,** not from what a sibling repo had. Repos drift — one may sit on `tilt 2.7.0` while another is already on `2.8.0` — and reusing the other's diff silently skips a whole release.

**A security advisory in a dependency is not automatically an exposure, and the question is settled by mechanism, not by name.** Read the advisory for the specific code path it names, then test whether this repo reaches *that*. Grepping for a plausible identifier and finding nothing proves nothing — it tests a name, not the property.

**Then check each behaviour change against actual usage before treating it as a risk.** A changed default or stricter validation only matters if this code reaches it.

Observed in a sibling repo: activesupport's CVE-2026-66066 was in **Active Storage**, which was not in that bundle at all. graphql's was `Marshal` deserialization in `GraphQL::Language::Cache#fetch` — settling that needed a check for a configured parser cache under *any* spelling, for `parse_file` calls, and for `Marshal` anywhere in graphql-client; a grep for `parser_cache` alone was worthless, and this repo *does* cache GraphQL data (a JSON schema dump), just not through the vulnerable path. tilt's new `.html.erb` registration was inert — no such files.


**Changelog sources, in the order that works:**
- `gh api repos/<owner>/<repo>/contents/<changelog>` piped through `base64 -d`, and `gh release view <tag> -R <owner>/<repo>`. Reliable, and reaches files WebFetch 404s on.
- `https://my.diffend.io/gems/<gem>/<from>/<to>` for gems — good when it works, but it sometimes returns only file statistics, or "we will start processing this job soon". Neither is a finding; fall back to `gh`.
- Don't guess raw GitHub paths. Changelog names vary (`HISTORY.md` not `HISTORY.rst`, `CHANGES` not `CHANGELOG`) — list the repo contents first.
- `gh api /advisories/<GHSA-id>` 404s. Read the repo's own `/security/advisories/<id>` page instead.

### 3. Note pinned gems

If any gems are pinned in the Gemfile (e.g., `gem 'yard', '0.9.37'`), check whether the pin is still needed by reviewing the git history for the pin commit and checking if the upstream issue is resolved. If a pin appears safe to remove, surface it as an explicit choice in the Step 4 approval question (e.g. an "Unpin and update yard" option) rather than removing the pin silently — the reason it was pinned is context the user may still care about.

### 4. Get user approval before updating

Present the findings table, then use `AskUserQuestion` (`multiSelect: true`) to let the user pick **which** gems to update — one option per outdated gem, each labeled with the gem name and target version, with the update type (security/bug/feature/data/housekeeping) in the description. Update only the selected gems; if nothing is selected, stop.

- `AskUserQuestion` allows at most 4 options, so per-gem selection only fits when ≤4 gems are outdated. When there are more, fall back to grouped choices — e.g. "Update all", "Security fixes only", "All except pinned" — and use the auto-added "Other" to let the user name specific gems to include or skip.
- If a pinned gem now looks safe to unpin (Step 3), include it as its own option (e.g. "Unpin and update yard").

Only proceed once the user has answered.

### 5. Run the update

If updating all outdated gems:

```bash
bundle update
```

If the user wants to skip specific gems, update only the approved ones:

```bash
bundle update gem1 gem2 gem3
```

Note: indirect dependencies (e.g., activesupport via graphql-client) can be listed by name — bundler will update them within their constraints.

### 6. Verify

Confirm the update succeeded and check for any warnings or errors in the output.

### 7. Commit

After verifying, use `AskUserQuestion` to gate the commit — **Commit `Gemfile.lock` now?** (e.g. `Commit` / `Not yet`). If confirmed, commit the `Gemfile.lock` change (plus the `Gemfile` if a pin was removed), highlighting security fixes in the commit message.

**No `Co-Authored-By: Claude` or `Claude-Session:` trailers in this repo** — see CLAUDE.md. Commits here follow FreeBSD ports conventions and feed patches submitted upstream.

**A committed lockfile is not a deployed one.** A pull on another node updates `Gemfile.lock` but not the installed gems; `bundle install` there is what makes the update real. This repo is checked out on **slowhand, okie and naturally** — the three FreeBSD nodes, because `rake testport` runs on all of them — so the unit of done is commit, push, then `git pull && bundle install` on the other two. Not on backless, which has no poudriere.

**Gems here are vendored per Ruby version** (`.bundle/config` sets `BUNDLE_PATH: ".vendor/bundle"`). A Ruby upgrade orphans the old `.vendor/bundle/ruby/<old>` tree, which `bundle clean` cannot see and will never remove; delete it by hand after reinstalling. Observed 2026-09-18: gems sat under `ruby/3.3` while slowhand had moved to 3.4, and every `rake` task failed with `Bundler::GemNotFound` until reinstalled.

## Common Mistakes

- Updating pinned gems without checking why they're pinned — always check git history first
- Not investigating the nature of updates before presenting them — the user wants to know what they're getting
- Running `bundle update` without user approval — gate the gem selection and the commit with `AskUserQuestion`, not free-text prompts
- Using `bundle update` without arguments when the user excluded specific gems — use `bundle update gem1 gem2 ...` to update only approved gems
- Removing a pin silently instead of surfacing it as an explicit "Unpin and update X" option in the Step 4 question
- Calling an update "routine" because the version bump looks small — read the changelog; patch releases carry security fixes
- Treating a pull on another host as the end of it — the lockfile moves, the installed packages don't until `bundle install` runs there
