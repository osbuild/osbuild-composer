================
osbuild-composer
================

------------------------
OSBuild Composer Service
------------------------

:Manual section: 7
:Manual group: Miscellaneous

DESCRIPTION
===========

The composer project is a set of HTTP services for composing operating system
images. It builds on the pipeline execution engine of *osbuild* [#osbuild]_ and
defines its own class of images that it supports building.

Multiple APIs are available to access a composer service. This includes
support for the *lorax-composer* [#lorax-github]_ API, and as
such can serve as drop-in replacement for lorax-composer.

You can control a composer instance either directly via the provided APIs, or
through higher-level user-interfaces from external projects. This, for
instance, includes a *Cockpit Module* [#cockpit-composer]_ or using the
*composer-cli* [#composer-cli]_ command-line tool.

Frontends
---------

*Composer* does not ship with frontends itself. However, several external
frontends for *Composer* already exist. These include:

**Cockpit Composer**
    This module for *Cockpit* [#cockpit]_ allows a great level of control of a
    *Composer* instance running on a cockpit-managed machine.

**Composer CLI**
    This command-line tool originated in the *lorax* [#lorax-github]_ project,
    but can be used with *Composer* just as well.

RUNNING
=======

To deploy a composer instance, all you need is to 

    |
    | # systemctl start osbuild-composer.socket
    |

Now you can access the service using `composer-cli`, for example:

    |
    | # composer-cli status show
    |

or using *Cockpit* with the *Cockpit Composer* module from a
browser: `http://localhost:9090`

SEE ALSO
========

``osbuild``\(1), ``osbuild-manifest``\(5)

NOTES
=====

.. [#osbuild] OSBuild:
              https://www.osbuild.org
.. [#lorax-github] Lorax Composer:
                   https://github.com/weldr/lorax
.. [#cockpit-composer] Cockpit Composer:
                       https://github.com/osbuild/cockpit-composer
.. [#composer-cli] Composer CLI:
                   https://weldr.io/lorax/composer-cli.html
.. [#cockpit] Cockpit Project:
              https://www.cockpit-project.org/
