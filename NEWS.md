# OSBuild Composer - Operating System Image Composition Services

## CHANGES WITH 34:

  * Support temporary AWS credentials
  * Composer-api and worker-api: OAuth2 support
  * Install docs in RHEL 8.5 and 9.0 images
  * RHEL-8.5 / RHEL-9.0: RHSM DNF plugins are now enabled by default on `ec2`
    and `ami` images
  * Added support for the following osbuild stages
    * `org.osbuild.selinux.config` - configures SELinux policy state and type
      on the system
    * `org.osbuild.tmpfilesd` - creates tmpfiles.d configuration files
    * `org.osbuild.pam.limits.conf` - creates configuration files for
       pam_limits module
    * `org.osbuild.sysctld` - creates sysctl.d configuration files
    * `org.osbuild.dnf.config` - configures DNF (currently only variables)
    * `org.osbuild.tuned` - sets active tuned profile (or more profiles)
  * More accurate HTTP return status for success requests in cloudapi


Contributions from: Alexander Todorov, Antonio Murdaca, Brian C. Lane,
                    Chloe Kaubisch, Diaa Sami, Gianluca Zuccarelli, Jakub Rusz,
                    Juan Abia, Martin Sehnoutka, Ondřej Budai, Simon Steinbeiss,
                    Thomas Lavocat, Tom Gundersen, Tomas Hozza, Sanne Raymaekers,
                    Yi He

— Berlin, 2021-09-15

## CHANGES WITH 33:

  * Bootiso: move payload to iso root
  * Fix building of RHEL 9.0 Edge images

Contributions from: Christian Kellner, Ondřej Budai, Xiaofeng Wang

— Liberec, 2021-08-30

## CHANGES WITH 32:

  * RHEL-Edge container image now uses nginx and serves on port 8080
  * Add support for RHEL 8.5 for Edge Raw images (x86_64 and Aarch64)
  * Add support for RHEL 8.5 Simplified installer (x86_64 and Aarch64)
  * Add custom file system support for RHEL 8.5
  * Add custom file system support for RHEL 9.0

Contributions from: Ondřej Budai, Alexander Todorov, Brian C. Lane, Tomas Hozza,
                    Chloe Kaubisch, Xiaofeng Wang, Achilleas Koutsou, Juan Abia,
                    Gianluca Zuccarelli, Diaa Sami, Christian Kellner,
                    Sanne Raymaekers, Jakub Rusz

— Vöcklabruck & Bruxelles, 2021-08-28

## CHANGES WITH 31:

  * Retrieve metadata about a compose through the Cloud API
  * Support multiple repository subscriptions
  * Support for building artifacts for Fedora 32 is now dropped
  * Add support for new / extended osbuild stages
  * Building images for other distributions
  * Add support for RHEL 8.5 Edge images
  * Add support for RHEL 8.5 main image types
  * Add support for RHEL 9.0 Beta
  * Weldr API: introduce the ability to limit exposed Image Types by configuration
  * Add a heartbeat between composer and the worker every 15 seconds.
  * Add oauth2 support to get composer into `cloud.redhat.com`.

Contributions from: Achilleas Koutsou, Alexander Todorov, Brian C. Lane,
                    Chloe Kaubisch, Colin Walters, Gianluca Zuccarelli,
                    Jakub Rusz, Juan Abia, Lars Karlitski, Martin Sehnoutka,
                    Ondřej Budai, Sanne Raymaekers, Thomas Lavocat, Tomas Hozza,
                    Xiaofeng Wang, Yuxin Sun

— Grenoble, 2021-08-11

## CHANGES WITH 30:

  * Cloud API now has support for uploading images directly to Azure. Before,
    composer only supported uploading to Azure using the Weldr API (used by
    cockpit-composer and composer-cli).

  * OSBuild Composer can now generate Manifests that conform to the new
    OSBuild schema. Two new image types are added that take advantage
    of the new schema: rhel-edge-container and rhel-edge-installer.

  * OSBuild Composer can now build RHEL 9.0 guest images.

  * OSBuild Composer can now build Fedora 34 and Fedora 35 images. Note that
    the work on Fedora 34 hasn't finished yet, stay tuned for more updates
    in the near future.

  * Cloud API has now support for authorization using an Identity header.
    This is useful for running composer in a Kubernetes cluster behind
    a reverse proxy.

  * Several fixes for RHEL 8.4 images are included in this release.

  * Worker osbuild jobs with GCP upload target now set the chosen image name as
    custom metadata on the uploaded object. This makes finding the uploaded
    object using the image name possible. The behavior is useful mainly
    for cleaning up cloud resources in case of unexpected failures.
  
  * Upload to GCP is now more reliable because composer now chooses a random
    GCE region instead of fixed one which seems to be often exhausted.

Contributions from: Achilleas Koutsou, Alexander Todorov, Brian C. Lane, Chloe
                    Kaubisch, Christian Kellner, Jakub Rusz, Jozef Mikovic,
                    Martin Sehnoutka, Ondřej Budai, Sanne Raymaekers, Tomas
                    Hozza, Tom Gundersen, Xiaofeng Wang, He Yi, Yuxin Sun

— Liberec, 2021-06-10

## CHANGES WITH 29:

  * Cloud API supports Google Cloud Platform as an upload target!

  * Cloud API now includes additional information about the upload target in
    response for "upload status". It works for both AWS and GCP.

  * The kernel osbuild-composer installs is now marked as the `saved_entry`
    making sure subsequent kernel installations do not change the default.

Contributions from: Alexander Todorov, Chloe Kaubisch, David Rheinsberg, Martin
                    Sehnoutka, Ondřej Budai, Sanne Raymaekers, Tomas Hozza, Tom
                    Gundersen

— Brno, 2021-03-04

## CHANGES WITH 28:

  * OSBuild Composer can now build CentOS Stream 8 images!

  * OSBuild Composer can now upload images to VMWare!

  * Cloudapi contains new /openapi.json and /version endpoints. The already
    existing `POST /compose` endpoint now supports specification of additional
    packages.

  * Blueprints now support definition of alternative kernels, such as kernel-rt,
    using the `customizations.kernel.name` key.

  * Attempt to use the `customization.kernel.append` key in a blueprint for the
    OSTree image type now results in error, because the customization was ignored.

  * OSBuild dependency now requires version 24 because of its new sysconfig and
    RHSM stages.

  * RHEL qcow2 images now come with disabled product-id and subscription-manager
    plugins in dnf to make them consistent with images produced by imagefactory.

  * OSTree `parent` can now be replaced with `url` pointing to a repository
    that contains the parent commit.

Contributions from: Achilleas Koutsou, Alexander Todorov, Antonio Murdaca,
                    Brian C. Lane, Chloe Kaubisch, Jacob Kozol, Jozef Mikovic,
                    Major Hayden, Martin Sehnoutka, Ondřej Budai, Sanne Raymaekers,
                    Tomas Hozza, Tom Gundersen

— Brno, 2021-02-20

## CHANGES WITH 27:

  * Starting from this release, we are putting longer news posts in
    `docs/news/*/`. We strongly encourage everyone to take a look at them!

  * OSBuild Composer now ships with a new osbuild-composer-core subpackage.
    It neither requires systemd nor a local worker, so it's perfect to use in
    a container. Just connect a remote worker to it and you are good to go!

  * The RHEL 8.4 guest image is now much closer to its original kickstart
    definition.
    
  * The API for Koji now supports retrieving manifests.

  * Uploaded AMIs now have a name tag, making them easier to search for in
    the AWS console.
    
  * The repository now contains a `Dockerfile` and `docker-compose.yml` for
    a quick setup of a development environment. Feel free to use it and let
    us know if you find it useful. We strongly recommend reading `HACKING.md`
    to learn about some caveats of this setup.
    
  * The bug allowing users to override a system repository in the Weldr API
    is now fixed.
    
  * The project now requires Go 1.14.

Contributions from: Achilleas Koutsou, Brian C. Lane, David Rheinsberg,
                    Jacob Kozol, Lars Karlitski, Major Hayden, Ondřej Budai,
                    Sanne Raymaekers, Tomas Hozza, Tom Gundersen

— Kořenov, 2020-02-04


## CHANGES WITH 26:

  * RHEL 8.4 images got plenty of updates:
    
    * Image building for aarch64, ppc64le and s390x is fixed.
    
    * The root XFS partition now has a random UUID. This change fixes image
      builds on an image built by osbuild-composer.
    
    * QCOW2 images are now closer to the old official ones:
    
      * The default size is now set to 10 GiB.
        
      * rng-tools are no longer installed.
    
      * kernel options are now aligned to the old official images.
    
    * org.osbuild.rhel84 runner is now used to build these images.
    
  * Worker crashed in a koji-finalize job when a previous koji-init job
    failed. This is now fixed.

  * Composer API has now support for mirrorlist and metalink.

  * Composer API now supports sharing an Amazon Machine Image with an another
    account.
    
  * Upload of aarch64 images to AWS is now fixed.

  * Composer API for Koji returns pending status until all images are
    finished. Previously, it returned failed as soon as the first image
    build failed.
    
  * Composer API for Koji and Worker API now log errors. This should
    very much simplify debugging.
    
  * osbuild-composer(7) man page is now included in the RPM.

  * The testing got some very nice updates too:

    * The CI now runs a subset of tests on Fedora 33 aarch64.
    
    * The CI now runs reverse dependency tests against koji-osbuild.

Contributions from: Chloe Kaubisch, Christian Kellner, Jacob Kozol,
                    Lars Karlitski, Ondřej Budai, Sanne Raymaekers,
                    Tomas Hozza

— Liberec, 2020-12-16

## CHANGES WITH 25:

  * Composer now supports RHEL 8.4! Big thanks to Jacob Kozol!
    If you want to build RHEL 8.4 using Composer API or Composer API for
    Koji, remember to pass "rhel-84" as a distribution name.

  * Composer can now be started without Weldr API. If you need it, start
    `osbuild-composer.socket` before `osbuild-composer.service` is started.
    Note that cockpit-composer starts `osbuild-composer.socket` so this change
    is backward compatible.
    
  * When Koji call failed, both osbuild-composer and osbuild-worker errored.
    This is now fixed.
    
  * The dependency on osbuild in the spec file is now moved to the worker
    subpackage. This was a mistake that could cause the worker to use
    an incompatible version of osbuild.
    
  * As always, testing got some upgrades. This time, mostly in the way
    we build our testing RPMs.

Contributions from: Jacob Kozol, Lars Karlitski, Ondřej Budai, Tom Gundersen

— Liberec, 2020-11-19

## CHANGES WITH 24:

  * Composer now internally supports multi-build composes. A big part of the
    jobqueue and worker was rewritten to support this feature.
    
  * Composer API for Koji was adjusted to use the new multi-build feature.
    All communication with Koji was moved to the worker so there's no need
    to have Koji credentials in composer (it's sufficient to have them in
    the worker). Additionally, the API can now correctly handle requests with
    multiple images.
    
  * Composer API for Koji has now /compose/{id}/logs route exposing logs to
    a caller. Keep in mind that the API specification doesn't guarantee the
    field structure, so it may change at any point in the future.

  * Composer API returned statuses that were not defined in the API
    specification. This is now fixed.
    
  * As always, there we are improvements in the testing pipeline. The biggest
    change is the introduction of Fedora 33 in composer's CI.

Contributions from: Chloe Kaubisch, Lars Karlitski, Martin Sehnoutka,
                    Ondřej Budai, Tom Gundersen

— Liberec, 2020-11-11

## CHANGES WITH 23:

  * Support for building Fedora 31 images was removed.

  * Metadata sent from Composer to Koji were adjusted based on a feedback
    from Koji maintainers. More fixes will definitely come in a future
    release.

  * Composer is now easier to deploy to OpenStack with a new deploy-openstack
    script available in the source tree. Note that the previous version
    introduced a similar tool for deploying a local qemu VM.
  
  * The testing setup is still being reworked massively. With the help
    of deploy-qemu, it should be now very simple to replicate the Schutzbot
    tests on a local machine.

Contributions from: Alexander Todorov, Lars Karlitski, Ondřej Budai, Tom
                    Gundersen, Xiaofeng Wang

— Liberec, 2020-11-04

## CHANGES WITH 22:

  * Support for building Fedora 33 images is now available as a tech preview.

  * The osbuild-composer-cloud binary is gone. The osbuild-composer binary
    now serves the Composer API along with Weldr and Koji APIs.

  * The testing setup was reworked. All files related to tests are now shipped
    in the tests subpackage. A script to run the test suite locally is now
    also available. See HACKING.md for more details.

  * GPG keys in Koji API are no longer marked as required.
    
  * Osbuild-composer RPM is now buildable on Fedora 33+ and Fedora ELN.

  * Osbuild-composer for Fedora 34 and higher now obsoletes lorax-composer.

Contributions from: Alexander Todorov, Jacob Kozol, Lars Karlitski,
                    Martin Sehnoutka, Ondřej Budai, Tom Gundersen

— Liberec, 2020-10-16

## CHANGES WITH 21:

  * Composer API is now available as a tech preview in the
    osbuild-composer-cloud subpackage. It's meant to be a simple API that
    allows users build an image and push it to a cloud provider. It doesn't
    support advanced features like storing blueprints as Weldr API does. This
    is not stable API, and is subject to incompatible change.

  * Koji API is now available in the -koji subpackage. It can be used
    to perform an image build and push the result directly to a Koji
    instance.
    
  * Worker API is now completely overhauled. Support for distinguishing
    architectures is added and the whole API is generated from an OpenAPI
    spec.
    
  * Weldr API's /projects/source/new route now explicitly requires the url
    field. 
  
  * The project now requires Go 1.13.

  * Testing of vmware and ostree images is now greatly improved.
  
  * All bash scripts are now checked with shellcheck on the CI.

Contributions from: Alexander Todorov, Lars Karlitski, Major Hayden,
                    Martin Sehnoutka, Ondřej Budai, Peter Robinson,
                    Sanne Raymaekers, Tom Gundersen, Xiaofeng Wang

— Liberec, 2020-09-24


## CHANGES WITH 20:

  * VMDK images are now stream optimized to be compatible with vCenter by
    defult.

  * RPMs are pulled from the correct repositories on RHEL, depending on whether
    the host is running on Beta or GA.

  * Cloud credentials can now no longer be returned by the API.

Contributions from: Alexander Todorov, Brian C. Lane, Lars Karlitski,
                    Major Hayden, Tom Gundersen

— London, 2020-08-23

## CHANGES WITH 19:

  * Bug fixes to the weldr API.

  * Default image size was increased to be able to build empty blueprints by
    default.

  * OpenStack images are now tested on the target footprint in CI.

  * Other test improvements.

Contributions from: Alexander Todorov, Brian C. Lane, Jenn Giardino,
                    Major Hayden, Martin Sehnoutka

— London, 2020-08-10

## CHANGES WITH 18:

  * Qcow and openstack images for Fedora have now cloudinit service enabled
    by default. This change leads to a higher consistency with the official
    images.
    
  * Fedora 32 image builds were failing if an installed package shipped
    a custom SELinux policy. This is now fixed.

  * The DNF integration now uses the fastestmirror plugin. This should lead
    to faster and more reliable depsolves.

  * Tar archives returned from Weldr routes could have contained files with
    a timestamp newer than the current time. This led to warnings when
    untarring these archives. The timestamps are now fixed.
    
  * The RCM subpackage was removed. It was never properly finished and will
    be superseded by a Koji integration at some point.

Contributions from: Chloe Kaubisch, Christian Kellner, David Rheinsberg,
                    Lars Karlitski, Major Hayden, Martin Sehnoutka,
                    Ondřej Budai, Tom Gundersen

— Liberec, 2020-07-22

## CHANGES WITH 17:

  * AWS images are now built in the raw format. Previously used vhdx was
    space-efficient but actually caused about 30% of uploads to fail.

  * The spec file had a wrong version of lorax-composer to obsolete, causing
    upgrades to fail. This is now fixed.

Contributions from: Major Hayden, Tom Gundersen

— Liberec, 2020-07-08

## CHANGES WITH 16:

  * osbuild-composer now obsoletes lorax-composer on RHEL.

  * An upload failure (e.g. due to invalid credentials) now causes the compose
    to appear as failed.
    
  * RHEL 8 repositories are switched to the beta ones to allow composer to be
    tested on 8.3 Beta. This will be reverted when GA comes.
  
  * OSTree images no longer contains /etc/fstab. The filesystem layout is
    determined by the installer and thus it doesn't make any sense to include
    it.
    
  * If both group and user customizations were used, the user would be created
    before the group, causing a build to fail. This is now fixed.
    
  * Composer now correctly passes UID and GID to org.osbuild.{users,groups}
    stages as ints instead of strings.

  * The subpackages (worker, tests and rcm) now require a matching version of
    osbuild-composer to be installed. Previously, they would be happy with
    just an arbitrary one.

  * Support for testing OpenStack images in actual OpenStack is now available.
    Note that upload to OpenStack is still not available for the end users
    (it's on the roadmap though).
    
  * Worker now logs not only job failures but also job successes.
  
  * All DNF errors were mistakenly tagged as RepoError, this is now fixed.
  
  * As always, a lot of test and CI improvements are included in this release.

Contributions from: Alexander Todorov, Christian Kellner, Major Hayden, Martin
                    Sehnoutka, Ondřej Budai, Tom Gundersen

— Liberec, 2020-06-29

## CHANGES WITH 15:

  * Support for building RHEL for Edge is now available.

  * Composer has now support for building QCOW2 and tar images for ppc64le and
    s390x architectures.
    
  * Tar images for RHEL have returned. The Image Builder team found out that
    they are used as a way to install RHEL for Satellite.

  * Blueprints containing packages with a wildcard version no longer causes
    the built image to have both x86_64 and i686 versions of one package
    installed.
  
  * GPG check is now disabled by default. If you have a custom
    repository in /etc/osbuild-composer/repositories, just set gpg_check
    to true to enable the check. Note that all the pre-defined repositories
    have GPG check enabled.
    
  * Composer now supports a cancellation of jobs. This can be done by calling
    /compose/cancel route of Weldr API.
   
  * osbuild-composer previously crashed when osbuild didn't return the right
    machine-readable output (e.g. because of a disk being out of space). This
    is now fixed.
    
  * Because of the GPG check change and RHEL for Edge support, composer
    now requires osbuild 17 or higher.
    
  * osbuild-composer previously required the python package to be installed
    on RHEL. Now, it uses the always-installed platform-python.

  * The buildroot for RHEL 8 didn't have selinux labels before. This is now
    fixed.
    
  * When Composer crashed, it left temporary directories in /var/cache. The
    temporary directories are now moved to /var/tmp, which is managed by
    systemd with PrivateTmp set to true, so they're now correctly removed
    after a crash.
    
  * Several weldr API routes were aligned to work in the same way as with
    Lorax. /blueprints/freeze now correctly supports option to output TOML.
    Projects and modules routes return all fields as Lorax returns.

  * AWS upload now logs the current state to the system journal. Emojis are
    of course included. 🎉
    
  * As always, amazing improvements in the CI infrastructure happened. Also,
    the test coverage went up. Thanks all for doing this!

Contributions from: Alexander Todorov, Brian C. Lane, Christian Kellner,
                    Jakub Rusz, Lars Karlitski, Major Hayden, Martin
                    Sehnoutka, Ondřej Budai, Peter Robinson, Tom
                    Gundersen

— Liberec, 2020-06-12

## CHANGES WITH 14:

  * AWS uploads doesn't anymore report to AWS that composer uploads
    the image in vhdx format. This surprisingly makes the upload process
    more stable.

  * Uploads were always in WAITING state. This is now fixed.
  
  * The /projects/source/* routes now correctly supports all the features
    of Weldr API v1. 

  * AWS upload now logs the progress to journal. Even better logging is
    hopefully coming soon.
    
  * AWS upload's status is now correctly set to FAILED when ImportSnapshot
    fails. Before, this hanged the upload indefinitely.

  * Store unmarshalling is now safer in some cases. For example, stored
    manifests are now longer checked when loaded from disk. Therefore,
    changing of manifest schema doesn't lead to crashes when old manifests
    are present in the store.
  
  * When store loading failed in non-verbose mode of osbuild-composer, it
    crashed the process because of nil logger. This is now fixed.

  * The upstream spec file for building osbuild-composer package now
    excludes the i686 architecture. Note that composer never supported
    this arch.
    
  * The upstream spec file now correctly specifies the composer's dependency
    to osbuild-ostree. This was forgotten in the previous release which
    introduced Fedora IoT support.

  * The previous version didn't have repositories defined for s390x and
    ppc64le architectures. This is now fixed. Note that this only fixes
    some codepaths, osbuild-composer still cannot build any images on
    these architectures. 

Contributions from: Brian C. Lane, Lars Karlitski, Major Hayden, Martin
                    Sehnoutka, Ondřej Budai, Stef Walter, Tom Gundersen

— Liberec, 2020-06-03

## CHANGES WITH 13:

  * Fedora IoT is now supported for Fedora 32 in the form of producing the
    commit tarball. Feel free to test it and report any issues you find.

  * Support for RHEL was completely revamped. Now, osbuild-composer supports
    building images only for the latest RHEL 8. The separate minor versions
    are no longer available. Additionally, it now uses the Red Hat CDN which
    requires the host system to be properly subscribed. If you need to use
    different package repositories to build RHEL from, use a repository
    override in /etc/osbuild-composer/repositories.
    
  * Several image types were removed: ext4-filesystem, partitioned-disk,
    and tar. The use-cases for these image types were not clearly defined and
    without a clear definition, it was very hard to define test cases for
    them.
    
  * Support for Fedora 30 was dropped as it is now EOL. So long and thanks
    for all the fish!

  * The timeout for AWS upload is removed. It's very hard to predict how long
    will the AWS upload take. With the timeout in place, it caused the test
    suite to produce a lot of false positives.
  
  * Build logs were broken in the previous release, this release fixes it.
    This time, they were properly saved but weldr API read them from a wrong
    location. This is now fixed and covered with basic tests.
    
  * Weldr API has now support for /compose/metadata and /compose/results
    routes. This allows users to easily access a manifest used to build
    an image.
    
  * Preliminary support for ppc64le and s390x is added to RHEL distribution.
    No images cannot be built yet but at least it won't crash on startup.
    
  * The weldr API socket has now correct permissions. As the result, it can
    be read and written only by root and the weldr group. This is the same
    behaviour as Lorax has.
    
  * By mistake, workers incorrectly used the default store for every build.
    However, this can currently cause the store to grow indefinitely, so
    this release switched the osbuild store to use a temporary directory again.
    
  * /status route in weldr API now correctly returns msgs field.

  * Handling of json (un)marshalling in store is revamped. It should
    make it more stable and simplify the maintenance of the store backwards
    compatibility.

  * Initial support for koji is now added. It's currently not hooked up
    to composer and only supports password authentication. More coming soon.
    
  * Again, the automated testing was greatly improved during this cycle,
    big thanks to everyone involved!

Contributions from: Alexander Todorov, Brian C. Lane, David Rheinsberg, Jacob
                    Kozol, Lars Karlitski, Major Hayden, Ondřej Budai, Tom
                    Gundersen


— Liberec, 2020-05-28

## CHANGES WITH 12:

  * In previous versions support for running remote workers was
    broken. This is now fixed and running remote workers is once
    again possible. See #568 for more information.

  * The job queue and the store are now two separate Go packages.
    One of the benefits is that it is now possible to build images
    without using the store which is too complicated for some usecases.

  * A blueprint name is now checked against the regex
    `^[a-zA-Z0-9._-]+$`. This is the same limitation as in
    lorax-composer.

  * All osbuild calls now use the new --output-directory argument.
    This change is a must because the old way of retrieving images from
    the osbuild store will soon be deprecated.

  * Some routes from the weldr API are now implemented in a more
    efficient way.

  * As always, the team worked hard on improving the tests and the CI.

Contributions from: Brian C. Lane, David Rheinsberg, Jiri Kortus, Lars
                    Karlitski, Major Hayden, Ondřej Budai

— Liberec, 2020-05-13

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

— Liberec, 2020-04-29

## CHANGES WITH 10:

  * The correct `metadata_expire` value is now passed to dnf. In the
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

— Liberec, 2020-04-15

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

— Liberec, 2020-04-01

## CHANGES WITH 8:

  * All generated pipelines now use the `org.osbuild.rpm` stage of
    osbuild, rather than `org.osbuild.dnf`. This improves on splitting
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

— Berlin, 2020-03-18

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

— Berlin, 2020-03-05

## CHANGES BEFORE 7:

  * Initial implementation of 'osbuild-composer'.

Contributions from: Alexander Todorov, Brian C. Lane, Christian Kellner,
                    Jacob Kozol, Jakub Rusz, Lars Karlitski, Martin
                    Sehnoutka, Ondřej Budai, Tom Gundersen
