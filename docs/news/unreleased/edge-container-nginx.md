# RHEL-Edge container image now uses nginx and serves on port 8080

Previously, the edge-container image type was unable to run in unprivileged
mode which prevented it from being used on OpenShift 4.  The container now uses
nginx to serve the commit and a configuration that allows it to run as a
non-root user inside the container.  The internal web server now uses port
`8080` instead of `80`.

See rhbz#1945238
