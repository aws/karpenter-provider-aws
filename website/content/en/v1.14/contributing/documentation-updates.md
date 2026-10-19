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
- Previews for your changes are built and available a few minutes after you push. Look for the "Amplify Preview URL" link in a comment in your PR.