# Release Process

Before changing `manifest.json`, creating a tag, building release artifacts, or publishing a GitHub release, state the proposed next version and justify it.

The version rationale must include:

- The current latest release and tag.
- The scope of the changes being released.
- Why the proposed `MAJOR.MINOR.PATCH` level fits semantic versioning expectations.
- The exact manifest `Version` value to use, including the fourth Elgato build component.
- The exact Git tag to create.

Wait for explicit approval of that version rationale before making any release-version changes or running release commands.

Issues are issues, in every workflow. A GitHub issue is the tracked work item with its own lifecycle, not a note, an internal document, or a passing remark: it is created (text approved first), labeled and assigned, referenced from the branch, commits, PRs, and release notes, updated in the issue itself on progress or blockers, and closed only as part of the publish flow after explicit approval. An issue is a reminder for work we will do later. Anything that can be fixed right now, or is already built, gets no issue: fix it, test it, commit it. Where an issue exists, internal documents under `docs/` do not replace it.

When a hardware test has been approved and the related change is committed and pushed, handle the corresponding GitHub issue as part of the same publish flow. Close it when the release/publish completes, or update it with the exact remaining blocker if it cannot be closed yet.

Release notes must follow the same structure as recent releases:

- Use a user-facing feature or "What's changed" heading.
- Group entries under sections such as "New features", "Improvements", or "Bug fixes" when useful.
- Always include a "Downloads" section listing both release artifacts.
- Do not include a validation section; validation belongs in the final assistant summary, not in public release notes.

Elgato Marketplace release notes are separate from GitHub release notes. The Marketplace listing is Windows-only for this plugin, so do not mention Linux, OpenDeck, release artifacts, downloads, validation, GitHub tags, commits, or hardware-test details there.

Use the existing Marketplace v1.7 style:

```markdown
## What's new

### Feature title
Short user-facing explanation.

### Feature title
Short user-facing explanation.

## Fixes
- Short fix or compatibility note.
```

For Marketplace updates, write only the Windows-relevant delta since the currently published Marketplace version.

### Finding the current Marketplace version

Do not ask which version is live on the Marketplace; look it up. The plugin's public listing is:

`https://marketplace.elgato.com/product/libre-hardware-monitor-af576388-8cbb-4d59-bdec-206dc3f4168e`

Fetch that page and read the **Version** field (it also shows the "Last Updated" date). That value is the currently published Marketplace version. Write the Marketplace changelog as the Windows-relevant delta between it and the version now being published. For example, if the listing shows `1.9.1` and you are publishing `2.0.0`, the changelog covers only what changed for Windows between 1.9.1 and 2.0.0.

## Binaries

Windows reputation services (Smart App Control, SmartScreen, Defender cloud protection) judge unsigned files by hash, so a release does not ship new hashes for `lhm.exe` and `lhm-bridge.exe` without a reason (issue #121, taken over from hwinfo-streamdeck). Security comes first: a known vulnerability in the old binaries is always a reason.

- All binaries are built with `-trimpath -buildvcs=false`. `make reproducible` builds `lhm.exe`, `lhm-bridge.exe` and the Linux `lhm` and `lhm-bridge` from two clones in different directories, one of them with an extra commit that touches no Go code, and fails when the bytes differ. CI runs it on every push and prints the hashes.
- The two Windows binaries are pure Go: the same source, dependencies and toolchain give the same bytes on every machine. The Linux binaries link against the C library of the machine that builds them, so they are the same between directories and commits on one machine but differ between machines. Everything below that depends on equal bytes covers the Windows binaries only.
- `make release` runs `scripts/release-binaries.sh` after the build. When the project's own Go code behind the two Windows binaries is unchanged since the previous release tag, the script puts that release's binaries back in the plugin directory, byte for byte, so only the plugin files (manifest, Property Inspector, images) differ. Test files are not compiled in and never count as a change. The Linux binaries are always the fresh build.
- The old binaries are only reused after `scripts/scan-release.sh` has downloaded them, found their SHA-256 equal to the one recorded for that release in `release-hashes.txt`, and scanned them with `govulncheck -mode=binary` without a finding. With a finding the freshly built binaries ship instead, so a dependency or toolchain update that fixes something in the old binaries goes out with this release. When the hashes are missing or different, or the download or the scanner fails, the release stops. It never falls back to reusing unverified binaries.
- Dependency and toolchain updates without a known vulnerability stay on `main` and go out with the next release that changes the Go code. `REBUILD_BINARIES=1 make release` ships fresh binaries regardless.
- `make release` records the hashes of the two Windows binaries it ships in `release-hashes.txt` under the version from the manifest, with the release they were built for. Run it before the release commit and commit that file with the release.
- The release workflow on GitHub builds the packages again from the tag. Because the version is tagged by then, it records nothing. It compares its Windows binaries with the committed hashes and stops when they differ.
- The binaries do not carry their commit. `make verify-release TAG=vX.Y.Z` rebuilds the Windows binaries of the release they were built for from its tag, with the toolchain and the flags of that tag, and compares the result with `release-hashes.txt`. Releases up to 2.1.1 were built without the flags and cannot be rebuilt this way.
- `scripts/scan-release.sh [tag]` downloads a release, compares its binaries with `release-hashes.txt` and scans them. Exit 0 is clean, 3 means known vulnerabilities, 1 anything else.
- `scripts/record-release-hashes.sh vX.Y.Z` records the hashes of the Linux binaries from the published packages, so the weekly scan can tell them apart from replaced assets. It stops when a Windows binary in the download differs from its recorded hash.
- `marketplace-version.txt` holds the GitHub release that is live on the Elgato Marketplace. Update it when a Marketplace submission is published.
- The "Release scan" workflow runs every week: the hash check and `govulncheck` on the latest GitHub release and on the Marketplace version, and the rebuild of the latest release. A failing job means a new release (or Marketplace update) is due, or the record does not match what is published.
- The Linux package bundles `lhm-companion`, built from its own repository with the toolchain in its `go.mod`. Keep that toolchain current there.

## Antivirus check

Every packed artifact is scanned before it is published (issue #93). `make release` and `make release-linux` each run `make av-check` on the package they just packed; a detection fails the build, and the release must not be published until the cause is understood. `make av-check` can also be run on its own (`AV_CHECK_PKG=<package>` selects the package, default the newest one in `build/`), and `scripts/av-check.sh <package>` on any package, for example a previously published asset after a definitions update.

The check scans the `.streamDeckPlugin` package and every executable directly inside its `.sdPlugin` directory (PE and ELF files, so `lhm.exe`, `lhm-bridge.exe`, `lhm`, `lhm-bridge` and `lhm-companion`, without a configured list):

- Microsoft Defender engine with the current definitions, offline, under wine64 (`scripts/defender-scan.sh`). This always runs and is the hard gate. It covers signatures and local heuristics only; Defender's cloud/ML verdicts (detection names ending in `!ml`) need a real Windows with Defender and cloud protection. It needs `wine64`, `x86_64-w64-mingw32-gcc`, `7z` and `curl`; the first run downloads about 220 MB of definitions into `~/.cache/defender-scan`, refreshed after 24 hours or with `--update`.
- The real Microsoft Defender, cloud protection included, in a disposable Windows 11 VM on QEMU/KVM (`scripts/defender-vm.sh`), run from Linux/WSL without anything on the Windows side. This is the only scan that produces Defender's cloud/ML verdicts. One-time setup: `scripts/defender-vm.sh create --iso <Windows 11 ISO>` installs Windows unattended and an OpenSSH server for host control, `prepare` updates the Defender platform and definitions and validates the cloud connection, `clean` freezes that disk as the baseline. Every `scan` boots a throwaway copy of the baseline, copies the files in (real-time protection judges them on write), updates definitions, runs `MpCmdRun.exe` without remediation, collects detections, versions and the Defender event log, and discards the copy. `make av-check` runs it automatically when the baseline exists (`~/.cache/defender-vm`, or `DEFENDER_VM_DIR`). Needs `qemu-system-x86_64` with `/dev/kvm`, `swtpm` and `xorrisofs`; the VM takes about 25 GB.
- VirusTotal hash lookup (`scripts/virustotal-file.js`) when `VIRUSTOTAL_APIKEY` is set; `AV_CHECK_FLAGS=--reanalyze` asks VirusTotal to rescan an already known file with the current engines.
- OPSWAT MetaDefender Cloud and Kaspersky OpenTIP (`scripts/av-online-check.js`) when `METADEFENDER_APIKEY` and `OPENTIP_APIKEY` are set.

The keys are read from the environment or from `~/.config/av-check/av.env` (`KEY=value` lines). That file is shared with hwinfo-streamdeck and any other project that uses this check, and stays outside the repositories. Override the path with `AV_CHECK_ENV`.

Nothing is uploaded unless `AV_CHECK_FLAGS=--upload` is passed; uploading shares the file with the vendors before the release exists. Reports are written to `build/av-reports/<package>-<timestamp>/` with a `summary.md`.

Scan the package, not only the executables: a signature can match the zip container while every file inside is clean.

### Reporting a false positive

No vendor offers an API for this; each has a form or an address. Report as the software developer, with the file, its SHA-256, the detection name and the source repository.

- Microsoft Defender: https://www.microsoft.com/en-us/wdsi/filesubmission (choose "Software developer"; track at https://www.microsoft.com/wdsi/submissionhistory)
- Bitdefender: https://www.bitdefender.com/submit/
- Kaspersky: https://opentip.kaspersky.com/ (submit for reanalysis) or newvirus@kaspersky.com
- Trend Micro: https://www.trendmicro.com/en_us/about/legal/detection-reevaluation.html
- Symantec/Broadcom: https://symsubmit.symantec.com/
- Sophos: https://support.sophos.com/support/s/filesubmission
- McAfee/Trellix: datasubmission@trellix.com
- ESET: samples@eset.com
- Avast: https://www.avast.com/report-false-positive; AVG: https://www.avg.com/false-positive-file-form
- Malwarebytes: https://forums.malwarebytes.com/forum/122-false-positives/
- F-Secure: https://www.f-secure.com/en/business/support-and-downloads/submit-a-sample

## Steps

1. Confirm the release scope and current latest release.
2. Propose and justify the next version.
3. Wait for explicit approval of the version rationale.
4. Update `Version` in `com.moeilijk.lhm.sdPlugin/manifest.json` to `MAJOR.MINOR.PATCH.0`.
5. Run `make release` and `make release-linux`. They decide which Windows binaries ship, record their hashes in `release-hashes.txt`, pack both artifacts and run the antivirus check on them. A detection stops the release.
6. Commit and push the release changes, `release-hashes.txt` included.
7. Handle the GitHub issue or issues included in the committed scope.
8. Create the GitHub release with tag `vMAJOR.MINOR.PATCH`, English release notes, and both artifacts attached:
   - `com.moeilijk.lhm-MAJOR.MINOR.PATCH.streamDeckPlugin`
   - `com.moeilijk.lhm-linux-MAJOR.MINOR.PATCH.streamDeckPlugin`
9. Confirm that the issue state matches the published release scope.
10. Run `scripts/record-release-hashes.sh vMAJOR.MINOR.PATCH` and `make verify-release TAG=vMAJOR.MINOR.PATCH`: the published Windows binaries must be the recorded ones and what the tag builds. Commit and push the Linux lines it added to `release-hashes.txt`.
