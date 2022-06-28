// package container implements a client for a container
// registry. It can be used to upload container images.
package container

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	_ "github.com/containers/image/v5/docker/archive"
	_ "github.com/containers/image/v5/oci/archive"
	_ "github.com/containers/image/v5/oci/layout"

	"github.com/containers/common/pkg/retry"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/transports"
	"github.com/containers/image/v5/types"
	"github.com/opencontainers/go-digest"
)

const (
	DefaultUserAgent  = "osbuild-composer/1.0"
	DefaultPolicyPath = "/etc/containers/policy.json"
)

type Credentials struct {
	Username string
	Password string
}

// A Client to interact with the given Target object at a
// container registry, like e.g. uploading an image to.
// All mentioned defaults are only set when using the
// NewClient constructor.
type Client struct {
	Target reference.Named // the target object to interact with
	Auth   Credentials     // credentials to use

	ReportWriter io.Writer // used for writing status reports, defaults to os.Stdout

	PrecomputeDigests bool // precompute digest in order to avoid uploads
	MaxRetries        int  // how often to retry http requests

	UserAgent string // user agent string to use for requests, defaults to DefaultUserAgent
	TlsVerify bool   // use an insecure connection

	// internal state
	policy *signature.Policy
	sysCtx *types.SystemContext
}

// NewClient constructs a new Client for target with default options.
// It will add the "latest" tag if target does not contain it.
func NewClient(target string) (*Client, error) {

	ref, err := reference.ParseNormalizedNamed(target)
	if err != nil {
		return nil, fmt.Errorf("failed to parse '%s': %w", target, err)
	}

	var policy *signature.Policy
	if _, err := os.Stat(DefaultPolicyPath); err == nil {
		policy, err = signature.NewPolicyFromFile(DefaultPolicyPath)
		if err != nil {
			return nil, err
		}
	} else {
		policy = &signature.Policy{
			Default: []signature.PolicyRequirement{
				signature.NewPRInsecureAcceptAnything(),
			},
		}
	}

	if err != nil {
		return nil, err
	}

	client := Client{
		Target: reference.TagNameOnly(ref),

		ReportWriter:      os.Stdout,
		PrecomputeDigests: true,

		UserAgent: DefaultUserAgent,
		TlsVerify: true,

		sysCtx: &types.SystemContext{
			RegistriesDirPath:        "",
			SystemRegistriesConfPath: "",
			BigFilesTemporaryDir:     "/var/tmp",
		},
		policy: policy,
	}

	return &client, nil
}

// SetCredentials will set username and password for Client
func (cl *Client) SetCredentials(username, password string) {
	cl.Auth.Username = username
	cl.Auth.Password = password
}

func parseImageName(name string) (types.ImageReference, error) {

	parts := strings.SplitN(name, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid image name '%s'", name)
	}

	transport := transports.Get(parts[0])
	if transport == nil {
		return nil, fmt.Errorf("unknown transport '%s'", parts[0])
	}

	return transport.ParseReference(parts[1])
}

// UploadImage takes an container image located at from and uploads it
// to the Target of Client. If tag is set, i.e. not the empty string,
// it will replace any previously set tag or digest of the target.
// Returns the digest of the manifest that was written to the server.
func (cl *Client) UploadImage(ctx context.Context, from, tag string) (digest.Digest, error) {

	targetCtx := *cl.sysCtx
	targetCtx.DockerInsecureSkipTLSVerify = types.NewOptionalBool(!cl.TlsVerify)
	targetCtx.DockerRegistryPushPrecomputeDigests = cl.PrecomputeDigests

	targetCtx.DockerAuthConfig = &types.DockerAuthConfig{
		Username: cl.Auth.Username,
		Password: cl.Auth.Password,
	}

	policyContext, err := signature.NewPolicyContext(cl.policy)

	if err != nil {
		return "", err
	}

	srcRef, err := parseImageName(from)
	if err != nil {
		return "", fmt.Errorf("invalid source name '%s': %w", from, err)
	}

	target := cl.Target

	if tag != "" {
		target = reference.TrimNamed(target)
		target, err = reference.WithTag(target, tag)
		if err != nil {
			return "", fmt.Errorf("error creating reference with tag '%s': %w", tag, err)
		}
	}

	destRef, err := docker.NewReference(target)
	if err != nil {
		return "", err
	}

	retryOpts := retry.RetryOptions{
		MaxRetry: cl.MaxRetries,
	}

	var manifestDigest digest.Digest

	err = retry.RetryIfNecessary(ctx, func() error {
		manifestBytes, err := copy.Image(ctx, policyContext, destRef, srcRef, &copy.Options{
			RemoveSignatures:      false,
			SignBy:                "",
			SignPassphrase:        "",
			ReportWriter:          cl.ReportWriter,
			SourceCtx:             cl.sysCtx,
			DestinationCtx:        &targetCtx,
			ForceManifestMIMEType: "",
			ImageListSelection:    copy.CopyAllImages,
			PreserveDigests:       false,
		})

		if err != nil {
			return err
		}

		manifestDigest, err = manifest.Digest(manifestBytes)

		return err

	}, &retryOpts)

	if err != nil {
		return "", err
	}

	return manifestDigest, nil
}
