# Bootiso: move payload to iso root

Instead of including the payload, i.e. ostree commits or live images,
in the anaconda squashfs, they are now located at the root of the iso.
This has several advantages, including shorter build times, more
flexibility in payload size and easier access to the actual payload.