# OSBuild Composer - Operating System Image Composition Services

## CHANGES WITH 12:

        * In previous versions support for running remote workers was
         broken. This is now fixed and running remote workers is once
         again possible. See #568 for more information.

        * The job queue and the store are now two separate Go packages.
         One of the benefits is that it is now possible to build images
         without using the store which is too complicated for some usecases.
         
        * A blueprint name is now checked against the regex
         "^[a-zA-Z0-9._-]+$". This is the same limitation as in
         lorax-composer.
         
        * All osbuild calls now use the new --output-directory argument.
         This change is a must because the old way of retrieving images from
         the osbuild store will soon be deprecated.
         
        * Some routes from the weldr API are now implemented in a more
         efficient way.
         
        * As always, the team worked hard on improving the tests and the CI.

        Contributions from: David Rheinsberg, Jiri Kortus, Lars Karlitski,
                            Major Hayden, Ondřej Budai

        - Liberec, 2020-05-13

## CHANGES WITH 11:

        * The support for uploading VHD images to Azure is now available.
        
        * AMI images are now produced in the vhdx format. This fixes
         the issue that those images couldn't be previously booted in EC2.

        * In version 10 the logs weren't saved when osbuild failed. This
         is now fixed.
         
        * The warnings when upgrading/removing the RPM package are now fixed.
         Note that updating to version 11 still produces them because
         the upgrade process runs also the scriptlets from version 10.
         
        * The size calculation for Fedora 31 vhd images is fixed.
        
        * The size field was removed from the tar assembler struct.
         The field has actually never been supported in osbuild
         and it doesn't make any sense.
        
        * The minimal required version of osbuild is bumped to 12.
        
        * This release also got big upgrades to the testing infrastructure,
         more tests are run on a CI and they now run faster. Also, the unit
         test coverage is improved.

        Contributions from: Alexander Todorov, Jacob Kozol, Jakub Rusz,
                            Jiri Kortus, Lars Karlitski, Major Hayden,
                            Ondřej Budai, Tom Gundersen

        - Liberec, 2020-04-29

## CHANGES WITH 10:

        * The correct metadata_expire value is now passed to dnf. In the
         past, this led to a lot of failed builds, because dnf has the
         default expire time set to 48 hours, whereas the Fedora updates
         repos have the expire time of 6 hours. 

        * A decision was made that the minimal Go version required for
         building the project is 1.12. This is now enforced by the CI.

        * The intermediate s3 object is now deleted after the upload to AWS
         is finished. It has no value for users.
         
        * The upload to AWS has now a bigger timeout. The current coronavirus
         situation is affecting the AWS responsiveness in a negative way.

        * The weldr API has better test coverage. In the process, several
         bugs in sources and composes were fixed.
        
        * Worker and jobqueue packages are receiving a big refactoring.
         This is the prerequisite for having multiple job queues for building
         images for different distributions and architectures.

        * The image tests now boot the AWS images in the actual EC2. 

        Contributions from: Alexander Todorov, Brian C. Lane,
                            Jacob Kozol, Jakub Rusz, Lars Karlitski,
                            Major Hayden, Martin Sehnoutka,
                            Ondřej Budai, Tom Gundersen

        - Liberec, 2020-04-15
        
## CHANGES WITH 9:

        * Fedora is now build with updates and modules repositories
         enabled, therefore up-to-date images are now produced.
         
        * A new man-page `osbuild-composer(7)` with high-level
         description of the project is now available. It can be built
         by the new man target in the Makfile.
         
        * All Fedora images have now a generic initramfs. This should
         make the images more reproducible and less likely failing to boot
         if the image build was done in a less usual environment.
         
        * Metalink is now used to access the Fedora repositories. This change
         should hopefully lead to more stable builds.

        * Composer is now released to Fedora 32 and 33 in a new
         osbuild-composer package. The old golang-github-osbuild-composer
         package will be automatically upgraded to the new one.

        * The internal osbuild-pipeline command now has a more user-friendly
         interface.

        * The RCM API (in development, experimental) is reworked to allow
         any distribution-architecture-image type combination.

        * The work on a high-level description of image types began.
         See image-types directory.
         
        * The osbuild-worker arguments are reworked, they are now much more
         flexible.
         
        * The image-info tool used in the integration tests can be now run
         on Fedora 32.
         
        * The unit test coverage is now much bigger, thanks to all
         contributors!
         
        * Internal distribution representation is significantly reworked,
         this simplifies the process of adding the support for all currently
         missing architectures.
         
        * Integration tests were also improved, the image tests are fully
         switched to the new Go implementation and an automatic way
         of generating test cases is added. The weldr API coverage is also
         much better. Several bugs in it were fixed in the process.
         
        * Codecov.io is now used to monitor the test coverage of the code.
         
        * As always, minor fixes and improvements all over the place.

        Contributions from: Alexander Todorov, Brian C. Lane, David
                            Rheinsberg, Jacob Kozol, Jakub Rusz, Jiri
                            Kortus, Lars Karlitski, Martin Sehnoutka,
                            Ondřej Budai, Tom Gundersen

        - Liberec, 2020-04-01

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
