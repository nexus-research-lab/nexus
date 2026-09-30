# macOS package signing readiness check

- Date: 2026-09-29
- Input: `desktop/macos/.build/app/Nexus.app`
- Architecture: arm64

The current local App is ad-hoc signed:

```text
CodeDirectory ... flags=0x2(adhoc)
Signature=adhoc
TeamIdentifier=not set
```

`spctl --assess` accepted it only with the local security override, and
`xcrun stapler validate` reported that no notarization ticket is stapled. This
is a readiness result, not a release acceptance result. Developer ID signing,
notarization, normal Gatekeeper assessment, clean-host installation, upgrade
and rollback remain unverified.
