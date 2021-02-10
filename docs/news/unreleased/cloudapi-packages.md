# Cloud API: The compose endopint now allow additional package selection

The `POST /compose` endpoint has now been extended to allow packages to
be requested in addition to the base ones for the image type. Packages
can only be requested by name, and the most recent ones that satisfy
dependency solving will be chosen.

Relevant PR:
https://github.com/osbuild/osbuild-composer/pull/1208
