# Cloud API: Add support for uploading to Azure

Cloud API now has support for uploading images directly to Azure. Before,
composer only supported uploading to Azure using the Weldr API (used by
cockpit-composer and composer-cli). Also, it only created a storage
blob requiring the user to do one extra step to run a VM.

The new Azure Image upload target creates a finished Azure Image that can
be immediately used to launch a VM. It also uses the Azure OAuth-based
authentication that doesn't require the user to give composer any credentials.

Note that this is currently only available for the Cloud API. If you are
a user of the Weldr API, you can still use the older method.
