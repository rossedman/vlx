## What and why

<!-- What does this change, and why? Link the issue it fixes. -->

## Checklist

- [ ] `make lint test cover` passes
- [ ] Behaviour changes are reflected in `docs/project/spec.md`
- [ ] New or changed commands have `Short`, `Long` and `Example` text, and the reference is regenerated (`make docs-reference`)
- [ ] Anything that sends a message goes through `safety`; anything that prints service data goes through `sanitize` or `render`
- [ ] Commit messages follow Conventional Commits
