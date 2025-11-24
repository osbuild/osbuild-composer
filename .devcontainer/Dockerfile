FROM fedora:44

# Install:
#  - a few packages for convenient usage
#  - RPM tooling
#  - the go compiler
#  - weldr-client to use the weldr API
#  - builddep to be able to pull in requirements from .spec
RUN dnf install -y \
    fish \
    fd-find \
    ripgrep \
    jq \
    fedora-packager \
    rpmdevtools \
    go-srpm-macros \
    go \
    weldr-client \
    osbuild \
    osbuild-lvm2 \
    osbuild-luks2 \
    osbuild-ostree \
    osbuild-tools \
    'dnf-command(builddep)'
# install the language server
RUN go install -v golang.org/x/tools/gopls@latest
RUN go install -v github.com/uudashr/gopkgs/v2/cmd/gopkgs@latest
RUN go install -v github.com/ramya-rao-a/go-outline@latest
RUN go install -v github.com/go-delve/delve/cmd/dlv@latest
RUN go install -v honnef.co/go/tools/cmd/staticcheck@latest
COPY ./osbuild-composer.spec /tmp/osbuild-composer.spec
RUN dnf builddep /tmp/osbuild-composer.spec -y
RUN rm /tmp/osbuild-composer.spec
