Designs
=======

These are the design documents for changes to Controller Runtime. They
exist to help document the design processes that go into writing
Controller Runtime, but may not be up-to-date (more below).

Not all changes to Controller Runtime need a design document -- only major
ones. Use your best judgement.

When submitting a design document, we encourage having written
a proof-of-concept, and it's perfectly acceptable to submit the
proof-of-concept PR simultaneously with the design document, as the
proof-of-concept process can help iron out wrinkles and can help with the
`Example` section of the template.

## Out-of-Date Designs

**Controller Runtime documentation
[GoDoc](https://pkg.go.dev/sigs.k8s.io/controller-runtime) should be
considered the canonical, update-to-date reference and architectural
documentation** for Controller Runtime.

However, if you see an out-of-date design document, feel free to submit
a PR marking it as such, and add an addendum linking to issues documenting
why things changed.  For example:

```markdown

# Out of Date

This change is out of date.  It turns out curly braces are frustrating to
type, so we had to abandon functions entirely, and have users specify
custom functionality using strings of Common LISP instead.  See #000 for
more information.
```
