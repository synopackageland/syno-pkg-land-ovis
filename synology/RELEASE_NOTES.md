# Ovis 0.4.22-0006

The first public DSM release of [Ovis](https://github.com/devswha/ovis), the
browser and phone client for herdr. Ovis is maintained by devswha; the DSM
package is distributed by Syno Package Land.

## What you get

- Access herdr workspaces, coding-agent conversations, prompts, approval cards
  and live terminals from a browser or phone.
- Connect your other computers over SSH from **Settings > Remote PCs**.
- Run local shells and agents inside the package's own Ubuntu container, with
  bundled herdr, Bun, Node.js and npm.
- Use common command-line tools, including Git, curl, jq, ripgrep, fd, rsync,
  SQLite and Python with pip and virtual environments.
- Keep the container's home data and workspace state across package stop/start.

## Requirements and sign-in

Install and start Container Manager before installing Ovis. This release
requires DSM 7.2.1-69057 or later and a compatible x86_64 or aarch64 NAS that
supports Container Manager. Choose the SPK for your NAS architecture.
Installation downloads the pinned container image from the internet; the SPK
does not include the image layers.

Open Ovis from the DSM menu or Package Center and sign in with a DSM
administrator account. The app opens on its own HTTPS page. All DSM
administrators share one Ovis identity, including its workspaces, credentials
and settings. There is no separate Ovis password or per-administrator isolation.
Other DSM users cannot enter.

## Runtime and privacy

Shells and agents run as the package account inside an unprivileged container.
The container has a read-only root filesystem and its own persistent data
volume, with no NAS Shared Folder or Docker socket mounted. Its memory limit is
2 GiB; large agent workloads may exceed that limit. Extra system packages and
compilers cannot be installed into the read-only image.

The container can access the internet for downloads, agents and SSH connections.
Ovis relay and Portal are disabled in this package. Anonymous usage counts are
off by default and can be enabled in **Settings > About**. Updates come through
DSM Package Center.

## Data retention

Package stop/start and subsequent container-based upgrades reuse the data volume.
Uninstalling retains that volume, including when the wizard's delete-data option
is selected. Keep this in mind when retiring an installation containing
credentials or agent history.

If you used an earlier native preview package, its data is not migrated into
the container automatically. Keep the old data for recovery and configure your
remote computers and agents again in the container.

## Versioning and license

Package versions use **upstream version-build**: `0.4.22-0006` packages upstream
Ovis `0.4.22`, build `0006`. The build number restarts with each upstream version.

Ovis and the DSM wrapper are MIT licensed. Bundled runtimes and other
dependencies retain their respective licenses and notices, included with the
package.
