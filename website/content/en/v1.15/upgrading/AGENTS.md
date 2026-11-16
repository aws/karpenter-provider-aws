# Upgrade guide notes

These rules apply when you add or edit the per-version notes in `upgrade-guide.md` (the `### Upgrading to <version>+` sections).

- Every breaking change needs an upgrade guide note, and the note starts with `**Breaking:**`.
- Order each version's notes by priority, which is how big the impact is on users who upgrade:
  1. Breaking changes, which affect everyone who upgrades. Keep them together at the top: behavior changes first, then metrics changes (whose impact is broken observability, e.g. dashboards and alerts). Put a change's escape hatch or migration note right after it.
  2. Notes that aren't breaking but still need action from some users, e.g. a changed default.
  3. Deprecations, which only matter if you use the deprecated feature.
  4. New opt-in feature gates, which only matter if you turn them on.

  Rank notes within each group by impact as well. For metrics, that means how much observability breaks, e.g. how widely the metric is used and how many queries the change breaks.
- This file isn't published: `website/hugo.yaml` lists `AGENTS.md` in `ignoreFiles`.
