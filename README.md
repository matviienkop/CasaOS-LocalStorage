# Shared RAID enumeration: screenshots

These screenshots were captured from the real CasaOS Storage Manager on a
Raspberry Pi 5 with four disks forming one Linux MD RAID10.

- `storage-before-v0.4.4.png`: original LocalStorage v0.4.4, four copies of md127.
- `storage-after-v0.4.4.png`: local v0.4.4 fix restored, one md127 storage entry.

The contribution in `fix/shared-raid-storage-enumeration` ports and improves this
fix against current upstream main. These screenshots illustrate the original
problem and the local fix; they do not claim deployment of the newer main-based
candidate. That candidate is validated separately with regression tests/builds.

Only the Storage Manager panel was captured. No UI content was altered.
The original service was temporarily restored for the before screenshot, then
the fixed binary was reinstated and its checksum verified. RAID stayed 4/4;
fstab and Samba configuration checksums were unchanged.

This branch contains review assets only and is not part of the code PR diff.
