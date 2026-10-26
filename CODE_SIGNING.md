# Code signing policy

Free code signing provided by [SignPath.io](https://about.signpath.io),
certificate by [SignPath Foundation](https://signpath.org).

## What is signed

Only Windows release artifacts built by GitHub Actions from this repository are
signed:

- `termimus-windows-amd64-portable.exe`
- `termimus-windows-amd64-installer.exe`

They are built from the tagged source on GitHub-hosted runners by
[`.github/workflows/release.yml`](.github/workflows/release.yml), and every
signing request must be approved manually before a signature is applied.
Builds made on developer machines, pull requests, and pushes to `main` are
never signed.

## Team roles

| Role | Members |
| --- | --- |
| Committers and reviewers | [rafliandi13](https://github.com/rafliandi13) |
| Approvers | [rafliandi13](https://github.com/rafliandi13) |

All team members use multi-factor authentication for GitHub and SignPath.

## Privacy policy

This program will not transfer any information to other networked systems
unless specifically requested by the user or the person installing or
operating it.

Termimus only connects to the SSH servers you add yourself. It has no
telemetry, analytics, crash reporting, or update checks. Hosts, passwords and
keys are stored encrypted on your own device (see the README); export files are
written only where you choose and are encrypted with a password you set.

## Maintainer setup

These steps are for the repository owner after SignPath Foundation accepts the
project.

1. In SignPath, create project `termimus` with a signing policy
   `release-signing` that requires manual approval, and connect it to this
   GitHub repository as a trusted build system.
2. Give the project this artifact configuration. Each GitHub artifact is a ZIP
   holding one `.exe`, which is Authenticode-signed:

   ```xml
   <?xml version="1.0" encoding="utf-8"?>
   <artifact-configuration xmlns="http://signpath.io/artifact-configuration/v1">
     <zip-file>
       <pe-file-set>
         <include path="*.exe" />
         <for-each>
           <authenticode-sign />
         </for-each>
       </pe-file-set>
     </zip-file>
   </artifact-configuration>
   ```

3. In GitHub → Settings → Secrets and variables → Actions:
   - Secret `SIGNPATH_API_TOKEN`: API token of a SignPath CI user with submitter
     rights on the project.
   - Variable `SIGNPATH_ORGANIZATION_ID`: your SignPath organization ID.
   - Optional variables `SIGNPATH_PROJECT_SLUG` (default `termimus`) and
     `SIGNPATH_POLICY_SLUG` (default `release-signing`).
4. Push a `v*` tag. The Windows job submits two signing requests (the exe, then
   the installer rebuilt around the signed exe); approve each one in SignPath.
