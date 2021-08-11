# Add support for RHEL 9.0 Beta

OSBuild Composer can now build RHEL 9.0 Beta images. All image types are based
off RHEL 8.5 ones, thus the same set of image types is supported.

Note that the test coverage isn't complete at this point. Fully supported is
just cross-building RHEL 9 qcow2 images on RHEL 8. Everything else is just
a technical preview.
