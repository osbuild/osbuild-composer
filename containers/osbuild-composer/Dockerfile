#
# osbuild-composer - Containerized OSBuild Composer
#
# This container provides a minimal fedora image with the osbuild-composer
# application installed and configured as default entrypoint.
#
# Build Arguments:
#
#   * OSB_FROM
#       This specifies the host image to use. It must be an RPM-based
#       distribution image with all osbuild-composer requirements
#       pre-installed.
#
#       Example: "docker.io/library/fedora:latest"
#
#   * OSB_RPMREPO
#       Base URL of an RPM repository from which to install osbuild-composer
#       from.
#
#       Example: "https://dl01.fedoraproject.org/pub/fedora/linux/releases/$releasever/Everything/$basearch/os/"
#

# Image arguments must be imported before `FROM`.
ARG             OSB_FROM="docker.io/library/fedora:latest"

# Prepare our host environment.
FROM            "${OSB_FROM}" AS base

# Import build parameters.
ARG             OSB_RPMREPO="https://dl01.fedoraproject.org/pub/fedora/linux/releases/\$releasever/Everything/\$basearch/os/"

# Create our state directory and use it as anchor.
WORKDIR         "/var/lib/osb"

# Create and switch into our src directory, which we use as temporary storage
# for all sources during the install.
WORKDIR         "./src"

# Install all global dependencies.
RUN \
                dnf \
                        -y \
                        "--repofrompath=ephemeral0,${OSB_RPMREPO}" \
                        "--setopt=ephemeral0.gpgcheck=0" \
                        "--setopt=ephemeral0.priority=10" \
                        install "osbuild-composer-core" \
                && dnf clean all

# Copy all our local sources, so we can access them from within the container
# build. They will be cleaned in a later step.
COPY            "." "."

# Prepare the runtime configuration and state.
RUN             mkdir -p "../bin"
RUN             mkdir -p "/etc/osbuild-composer/"
RUN             mkdir -p "/run/osbuild-composer/"
RUN             mkdir -p "/run/weldr/"
RUN             mkdir -p "/var/cache/osbuild-composer/"
RUN             mkdir -p "/var/cache/osbuild-worker/"
RUN             mkdir -p "/var/lib/osbuild-composer/"

# Install all required sources into the persistent directory.
RUN             cp "entrypoint.py" "../bin/"

# Leave and delete our temporary source directory.
WORKDIR         ".."
RUN             rm -rf "./src"

# Prepare the runtime entrypoint and empty working directory.
WORKDIR         "./workdir"
ENTRYPOINT      ["python3", "../bin/entrypoint.py"]
