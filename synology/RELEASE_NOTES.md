# Ovis 0.5.1-0001

Upgrade to upstream [Ovis v0.5.1](https://github.com/devswha/ovis/releases/tag/v0.5.1),
including the changes in v0.5.0. Ovis is maintained by devswha; the DSM package
is distributed by Syno Package Land.

## Changes

- Find SSH connections under **Settings > Devices and PCs > Add a connection > SSH**.
  The Add PC dialog retains the package's SSH guidance and **Copy public key** action.
- Receive upstream improvements to agent conversations, conversation search,
  file viewing, agent grouping and alerts. Version 0.5.1 also fixes terminal
  cursor movement, Safari sidebar height and phone file scrolling.
- Repair SSH account lookup and retained-home ownership when upgrading from
  0.4.22-0006. Existing private keys, SSH aliases and workspace data are retained.
- Rebuild both runtime architectures from the unmodified v0.5.1 source, using
  SDK 0.53.9 and CLI 0.52.6, with an immutable container image digest.

## Requirements and sign-in

Requires DSM 7.2.1-69057 or later, Container Manager installed and running,
and a compatible x86_64 or aarch64 NAS. Select the matching SPK. Installation
downloads the pinned runtime image from the internet; image layers are not
included in the SPK.

Open Ovis from the DSM menu or Package Center using a DSM administrator session.
All administrators share the same Ovis identity, home, credentials and settings.
There is no separate Ovis password. The package does not require executor
approval or an additional DSM password confirmation. Standard third-party
package consent and Ovis's remote bridge confirmation still apply. Remote PCs
retain their own SSH authentication and host-key verification.

## Runtime and privacy

Shells, agents and SSH run as root inside the package's Ubuntu container, with
HOME and root's passwd home set to `/data/home`. Without user namespace remapping,
container UID 0 is also host UID 0. Host package control and the gateway run as
the DSM package account.

The container has a read-only root filesystem, no-new-privileges, dropped
capabilities with DAC_OVERRIDE and CHOWN explicitly enabled for retained home
migration, and its own named data volume. Container Manager may additionally
supply NET_BIND_SERVICE. No NAS Shared Folder or Docker socket is mounted.
The memory limit is 2 GiB. The image includes common shell tools and Python
with pip and virtual environments; its system filesystem cannot be modified.

The container has internet access for agents, downloads and SSH. Relay and
Portal remain disabled in this package, including upstream's account relay
connection flow. Anonymous usage counts start off and remain user-controlled
under Settings > About. Updates come through Package Center.

## Retained data

Upgrades reuse the data volume. Uninstalling retains it even when the wizard's
delete-data option is selected; retained data may contain credentials and
agent history. The ownership migration stays within the private home and does
not follow symlinks. Earlier native preview data is not imported automatically.

The package version is upstream version plus packaging build; the build resets
to 0001 for this upstream upgrade. Ovis and the DSM wrapper are MIT licensed;
bundled dependencies retain their included licenses and notices.
