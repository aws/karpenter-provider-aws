---
title: "Documentation Updates"
linkTitle: "Documentation Updates"
weight: 50
description: >
  Information helpful for contributing simple documentation updates.
---

- Documentation for https://karpenter.sh/docs/ is built under website/content/en/preview/.
- Documentation updates for an unreleased change should be made to the "preview" directory. Your changes will be promoted to website/content/en/docs/ by an automated process at the next release (not when the change merges).
- Fixes or corrections to already-released documentation should also be applied directly to website/content/en/docs/ (so they appear on the live site before the next release) and backported to every other affected version under website/content/en/ *besides* the /docs/ folder.
- Your site is built automatically when you push a change under `website/`, but the preview is only deployed on request: a maintainer leaves a `/karpenter preview` review comment on your PR and the preview URL is posted as a comment a few minutes later. Pushing a new commit does not refresh the preview, so ask for another `/karpenter preview` once you have made changes. The preview is built from the exact commit the maintainer reviewed, so if your most recent commit changes nothing under `website/` there is no build to publish — push a `website/` change and ask against that commit. Builds are kept for 14 days.
