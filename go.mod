module github.com/osbuild/osbuild-composer

go 1.16

exclude github.com/mattn/go-sqlite3 v2.0.3+incompatible

require (
	cloud.google.com/go/cloudbuild v1.2.0
	cloud.google.com/go/compute v1.7.0
	cloud.google.com/go/storage v1.22.1
	github.com/Azure/azure-pipeline-go v0.2.3
	github.com/Azure/azure-sdk-for-go v66.0.0+incompatible
	github.com/Azure/azure-storage-blob-go v0.13.0
	github.com/Azure/go-autorest/autorest v0.11.27
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.11
	github.com/BurntSushi/toml v1.2.0
	github.com/aws/aws-sdk-go v1.44.44
	github.com/containers/common v0.48.0
	github.com/containers/image/v5 v5.22.0
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f
	github.com/deepmap/oapi-codegen v1.8.2
	github.com/getkin/kin-openapi v0.93.0
	github.com/go-openapi/swag v0.21.1 // indirect
	github.com/gobwas/glob v0.2.3
	github.com/golang-jwt/jwt/v4 v4.4.1
	github.com/google/go-cmp v0.5.8
	github.com/google/uuid v1.3.0
	github.com/gophercloud/gophercloud v0.24.0
	github.com/hashicorp/go-retryablehttp v0.7.1
	github.com/jackc/pgtype v1.11.0
	github.com/jackc/pgx/v4 v4.16.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kolo/xmlrpc v0.0.0-20201022064351-38db28db192b
	github.com/labstack/echo/v4 v4.7.2
	github.com/labstack/gommon v0.3.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.3-0.20220114050600-8b9d41f48198
	github.com/openshift-online/ocm-sdk-go v0.1.266
	github.com/oracle/oci-go-sdk/v54 v54.0.0
	github.com/prometheus/client_golang v1.12.1
	github.com/segmentio/ksuid v1.0.4
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/cobra v1.4.0
	github.com/stretchr/testify v1.8.0
	github.com/ubccr/kerby v0.0.0-20170626144437-201a958fc453
	github.com/vmware/govmomi v0.28.0
	golang.org/x/oauth2 v0.0.0-20220622183110-fd043fe589d2
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8
	google.golang.org/api v0.86.0
	google.golang.org/genproto v0.0.0-20220624142145-8cd45d7dbd1f
	gopkg.in/ini.v1 v1.66.6
)
