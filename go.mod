module github.com/osbuild/osbuild-composer

go 1.16

require (
	cloud.google.com/go/cloudbuild v1.2.0
	cloud.google.com/go/compute v1.6.0
	cloud.google.com/go/storage v1.22.0
	github.com/Azure/azure-pipeline-go v0.2.3
	github.com/Azure/azure-sdk-for-go v63.1.0+incompatible
	github.com/Azure/azure-storage-blob-go v0.14.0
	github.com/Azure/go-autorest/autorest v0.11.27
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.11
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/BurntSushi/toml v1.1.0
	github.com/aws/aws-sdk-go v1.44.1
	github.com/cenkalti/backoff/v4 v4.1.1 // indirect
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f
	github.com/deepmap/oapi-codegen v1.8.2
	github.com/getkin/kin-openapi v0.61.0
	github.com/gobwas/glob v0.2.3
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.5.7
	github.com/google/uuid v1.3.0
	github.com/gophercloud/gophercloud v0.24.0
	github.com/hashicorp/go-retryablehttp v0.7.1
	github.com/jackc/pgtype v1.10.0
	github.com/jackc/pgx/v4 v4.15.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kolo/xmlrpc v0.0.0-20201022064351-38db28db192b
	github.com/labstack/echo/v4 v4.7.2
	github.com/labstack/gommon v0.3.1
	github.com/openshift-online/ocm-sdk-go v0.1.214
	github.com/oracle/oci-go-sdk/v54 v54.0.0
	github.com/prometheus/client_golang v1.12.1
	github.com/segmentio/ksuid v1.0.4
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.4.0
	github.com/stretchr/testify v1.7.1
	github.com/ubccr/kerby v0.0.0-20170626144437-201a958fc453
	github.com/vmware/govmomi v0.27.4
	golang.org/x/oauth2 v0.0.0-20220411215720-9780585627b5
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad
	google.golang.org/api v0.75.0
	google.golang.org/genproto v0.0.0-20220414192740-2d67ff6cf2b4
	gopkg.in/ini.v1 v1.66.4
)
