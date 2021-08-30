# Fix building of RHEL 9.0 Edge images

RHEL 9.0 Beta doesn't ship iwl6000-firmware anymore therefore we had to remove
it from the edge-commit and edge-installer image definitions.
