This directory includes 3rd party modules, needed in CI.

 - [`dnsname`](https://github.com/containers/dnsname) plugin for podman,
   needed to translate host names of containers into IPs. It is shipped
   in Fedora, but missing in RHEL 8, see
   [rhgbz#1877865](https://bugzilla.redhat.com/show_bug.cgi?id=1877865).
   The `87-podman-bridge.conflist` file contains the corresponding config,
   where the `{"domainName": "dns.podman", "type": "dnsname"}` bit is the
   newly added part.
