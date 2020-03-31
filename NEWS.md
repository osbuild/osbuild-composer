# OSBuild Composer - Operating System Image Composition Services

## CHANGES WITH 8:

        * All generated pipelines now use the `org.osbuild.rpm` stage of
          *osbuild*, rather than `org.osbuild.dnf`. This improves on splitting
          resource acquisition from image building and should make image
          composition more reliable and faster.

        * The `STATE_DIRECTORY` environment variable now allows changing the
          state directory path of `osbuild-composer`. This is to support older
          systemd versions that do not pass in `StateDirectory=` to the service
          executable.

        * Minor fixes and improvements all over the place.

        Contributions from: Alexander Todorov, Brian C. Lane, Jacob Kozol, Jakub
                            Rusz, Lars Karlitski, Major Hayden, Martin
                            Sehnoutka, Ondřej Budai, Tom Gundersen

        - Berlin, 2020-03-18

## CHANGES WITH 7:

        * Support for `RHEL 8.1` as image type is now available.

        * Semantic versioning of blueprints in the lorax API is now enforced.
          This was always the case for the original lorax API, and *Composer*
          now follows this as well.

        * Lots of internal improvements, including many automatic tests,
          improved error handling, better cache directory management, as well
          as preparations to move over from `org.osbuild.dnf` to
          `org.osbuild.rpm` in all build pipelines.

        Contributions from: Alexander Todorov, Brian C. Lane, Jacob Kozol, Lars
                            Karlitski, Major Hayden, Ondřej Budai, Tom Gundersen

        - Berlin, 2020-03-05

## CHANGES BEFORE 7:

        * Initial implementation of 'osbuild-composer'.

        Contributions from: Alexander Todorov, Brian C. Lane, Christian Kellner,
                            Jacob Kozol, Jakub Rusz, Lars Karlitski, Martin
                            Sehnoutka, Ondřej Budai, Tom Gundersen
