# Install docs in RHEL 8.5 and 9.0 images

Previously, all packages in all image types were installed using the
--excludedocs options. This is great for the image size but it actually
causes some issues too: The biggest one is that there are no man pages inside
the images. As that is a pretty big regression, we decided to revert
the --excludedocs setting now.
