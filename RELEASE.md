# Release Process

Before changing `manifest.json`, creating a tag, building release artifacts, or publishing a GitHub release, state the proposed next version and justify it.

The version rationale must include:

- The current latest release and tag.
- The scope of the changes being released.
- Why the proposed `MAJOR.MINOR.PATCH` level fits semantic versioning expectations.
- The exact manifest `Version` value to use, including the fourth Elgato build component.
- The exact Git tag to create.

Wait for explicit approval of that version rationale before making any release-version changes or running release commands.

When a hardware test has been approved and the related change is committed and pushed, handle the corresponding GitHub issue as part of the same publish flow. Close it when the release/publish completes, or update it with the exact remaining blocker if it cannot be closed yet.

## Steps

1. Confirm the release scope and current latest release.
2. Propose and justify the next version.
3. Wait for explicit approval of the version rationale.
4. Update `Version` in `com.moeilijk.lhm.sdPlugin/manifest.json` to `MAJOR.MINOR.PATCH.0`.
5. Commit and push the release changes.
6. Handle the GitHub issue or issues included in the committed scope.
7. Run `make release`.
8. Run `make release-linux`.
9. Create the GitHub release with tag `vMAJOR.MINOR.PATCH`, English release notes, and both artifacts attached:
   - `com.moeilijk.lhm.streamDeckPlugin`
   - `com.moeilijk.lhm-linux.streamDeckPlugin`
10. Confirm that the issue state matches the published release scope.
