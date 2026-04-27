# UI Behavior Docs

This documentation captures the framework-level UI behaviors of `foxpro-go`.

- **`docs/foundation-ui-spec.md`** — full behavior spec with
  `[done]` / `[planned]` / `[future]` markers per item.
- **`docs/building-apps.md`** — guide for developers consuming the
  framework: quick start, supported API surface, extension patterns,
  the line between consumer code and framework internals.
- **`docs/CHANGELOG.md`** — checkpoint snapshots of what's working at a
  given milestone.
- **`docs/wishlist.md`** — framework patterns proposed but not yet
  built. App authors who hit a missing primitive must add an entry
  here (see *Contributing Patterns Back* in `building-apps.md`)
  rather than working around it in their app.

Application-level specs (menu structures, dialogs, commands specific to a
consumer app like a Kubernetes browser or git client) belong in that app's
own repository on top of this foundation.
