FROM fedora:34

# Install few packages for convenient usage.
# Inspired by the devcontainer in osbuild.
RUN dnf install -y \
    fish \
    fd-find \
    ripgrep \
    jq
# We build composer using RPM, install the tooling
RUN dnf install fedora-packager rpmdevtools go-srpm-macros -y
# The list of requirements is specified in the spec file.
# To install them, dnf needs the "builddep" command.
RUN dnf install 'dnf-command(builddep)' -y
# Copy the specfile, remove gometa because dnf fails
# to parse it, and finally install the dependencies.
COPY osbuild-composer.spec /tmp/osbuild-composer.spec
RUN dnf builddep /tmp/osbuild-composer.spec -y
