package koji

import (
	"bytes"
	"context"

	// koji uses MD5 hashes
	/* #nosec G501 */
	"crypto/md5"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"hash/adler32"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	rh "github.com/hashicorp/go-retryablehttp"
	"github.com/kolo/xmlrpc"
	"github.com/sirupsen/logrus"
	"github.com/ubccr/kerby/khttp"

	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type Koji struct {
	xmlrpc    *xmlrpc.Client
	server    string
	transport http.RoundTripper
}

// BUILD METADATA

// TypeInfoBuild is a map whose entries are the names of the build types
// used for the build, and the values are free-form maps containing
// type-specific information for the build.
type TypeInfoBuild struct {
	// Image holds extra metadata about all images built by the build.
	// It is a map whose keys are the filenames of the images, and
	// the values are the extra metadata for the image.
	// There can't be more than one image with the same filename.
	Image map[string]ImageExtraInfo `json:"image"`
}

// BuildExtra holds extra metadata associated with the build.
// It is a free-form map, but must contain at least the 'typeinfo' key.
type BuildExtra struct {
	TypeInfo TypeInfoBuild `json:"typeinfo"`
	// Manifest holds extra metadata about osbuild manifests attached to the build.
	// It is a map whose keys are the filenames of the manifests, and
	// the values are the extra metadata for the manifest.
	Manifest map[string]*ManifestExtraInfo `json:"osbuild_manifest,omitempty"`
}

// Build represents a Koji build and holds metadata about it.
type Build struct {
	BuildID   uint64 `json:"build_id"`
	TaskID    uint64 `json:"task_id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Release   string `json:"release"`
	Source    string `json:"source"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	// NOTE: This is the struct that ends up shown in the buildinfo and webui in Koji.
	Extra BuildExtra `json:"extra"`
}

// BUIDROOT METADATA

// Host holds information about the host where the build was run.
type Host struct {
	Os   string `json:"os"`
	Arch string `json:"arch"`
}

// ContentGenerator holds information about the content generator which run the build.
type ContentGenerator struct {
	Name    string `json:"name"` // Must be 'osbuild'.
	Version string `json:"version"`
}

// Container holds information about the container in which the build was run.
type Container struct {
	// Type of the container that was used, e.g. 'none', 'chroot', 'kvm', 'docker', etc.
	Type string `json:"type"`
	Arch string `json:"arch"`
}

// Tool holds information about a tool used to run build.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// BuildRoot represents a buildroot used for the build.
type BuildRoot struct {
	ID               uint64           `json:"id"`
	Host             Host             `json:"host"`
	ContentGenerator ContentGenerator `json:"content_generator"`
	Container        Container        `json:"container"`
	Tools            []Tool           `json:"tools"`
	RPMs             []rpmmd.RPM      `json:"components"`
}

// OUTPUT METADATA

type ImageOutputTypeExtraInfo interface {
	isImageOutputTypeMD()
}

// ImageExtraInfo holds extra metadata about the image.
// This structure is shared for the Extra metadata of the output and the build.
type ImageExtraInfo struct {
	// Koji docs say: "should contain IDs that allow tracking the output back to the system in which it was generated"
	// TODO: we should probably add some ID here, probably the OSBuildJob UUID?

	Arch string `json:"arch"`
	// Boot mode of the image
	BootMode string `json:"boot_mode,omitempty"`
	// Configuration used to prouce this image using osbuild
	OSBuildArtifact *target.OsbuildArtifact `json:"osbuild_artifact,omitempty"`
	// Version of the osbuild binary used by the worker to build the image
	OSBuildVersion string `json:"osbuild_version,omitempty"`
	// Results from any upload targets associated with the image
	// except for the Koji target.
	UploadTargetResults []*target.TargetResult `json:"upload_target_results,omitempty"`
}

func (ImageExtraInfo) isImageOutputTypeMD() {}

type OSBuildComposerDepModule struct {
	Path    string                    `json:"path"`
	Version string                    `json:"version"`
	Replace *OSBuildComposerDepModule `json:"replace,omitempty"`
}

// ManifestInfo holds information about the environment in which
// the manifest was produced and which could affect its content.
type ManifestInfo struct {
	OSBuildComposerVersion string `json:"osbuild_composer_version"`
	// List of relevant modules used by osbuild-composer which
	// could affect the manifest content.
	OSBuildComposerDeps []*OSBuildComposerDepModule `json:"osbuild_composer_deps,omitempty"`
}

// ManifestExtraInfo holds extra metadata about the osbuild manifest.
type ManifestExtraInfo struct {
	Arch string        `json:"arch"`
	Info *ManifestInfo `json:"info,omitempty"`
}

func (ManifestExtraInfo) isImageOutputTypeMD() {}

type SbomDocExtraInfo struct {
	Arch string `json:"arch"`
}

func (SbomDocExtraInfo) isImageOutputTypeMD() {}

// BuildOutputExtra holds extra metadata associated with the build output.
type BuildOutputExtra struct {
	// ImageOutput holds extra metadata about a single "image" output.
	// "image" in this context is the "build type" in the Koji terminology,
	// not necessarily an actual image. It can and must be used also for
	// other supplementary files related to the image, such as osbuild manifest.
	// The only exception are logs, which do not need to specify any "typeinfo".
	ImageOutput ImageOutputTypeExtraInfo `json:"image"`
}

// BuildOutputType represents the type of a BuildOutput.
type BuildOutputType string

const (
	BuildOutputTypeImage    BuildOutputType = "image"
	BuildOutputTypeLog      BuildOutputType = "log"
	BuildOutputTypeManifest BuildOutputType = "osbuild-manifest"
	BuildOutputTypeSbomDoc  BuildOutputType = "sbom-doc"
)

// ChecksumType represents the type of a checksum used for a BuildOutput.
type ChecksumType string

const (
	ChecksumTypeMD5     ChecksumType = "md5"
	ChecksumTypeAdler32 ChecksumType = "adler32"
	ChecksumTypeSHA256  ChecksumType = "sha256"
)

// BuildOutput represents an output from the OSBuild content generator.
// The output can be a file of various types, which is imported to Koji.
// Examples of types are "image", "log" or other.
type BuildOutput struct {
	BuildRootID  uint64            `json:"buildroot_id"`
	Filename     string            `json:"filename"`
	FileSize     uint64            `json:"filesize"`
	Arch         string            `json:"arch"` // can be 'noarch' or a specific arch
	ChecksumType ChecksumType      `json:"checksum_type"`
	Checksum     string            `json:"checksum"`
	Type         BuildOutputType   `json:"type"`
	RPMs         []rpmmd.RPM       `json:"components,omitempty"`
	Extra        *BuildOutputExtra `json:"extra,omitempty"`
}

// CONTENT GENERATOR METADATA

// Metadata holds Koji Content Generator metadata.
// This is passed to the CGImport call.
// For more information, see https://docs.pagure.org/koji/content_generator_metadata/
type Metadata struct {
	MetadataVersion int           `json:"metadata_version"` // must be '0'
	Build           Build         `json:"build"`
	BuildRoots      []BuildRoot   `json:"buildroots"`
	Outputs         []BuildOutput `json:"output"`
}

// KOJI API STRUCTURES

type CGInitBuildResult struct {
	BuildID int    `xmlrpc:"build_id"`
	Token   string `xmlrpc:"token"`
}

type CGImportResult struct {
	BuildID int `xmlrpc:"build_id"`
}

type GSSAPICredentials struct {
	Principal string
	KeyTab    string
}

type loginReply struct {
	SessionID  int64  `xmlrpc:"session-id"`
	SessionKey string `xmlrpc:"session-key"`
}

func newKoji(server string, transport http.RoundTripper, reply loginReply) (*Koji, error) {
	// Create the final xmlrpc client with our custom RoundTripper handling
	// sessionID, sessionKey and callnum
	kojiTransport := &Transport{
		sessionID:  reply.SessionID,
		sessionKey: reply.SessionKey,
		callnum:    0,
		transport:  transport,
	}

	client, err := xmlrpc.NewClient(server, kojiTransport)
	if err != nil {
		return nil, err
	}

	return &Koji{
		xmlrpc:    client,
		server:    server,
		transport: kojiTransport,
	}, nil
}

// NewFromPlain creates a new Koji sessions  =authenticated using the plain
// username/password method. If you want to speak to a public koji instance,
// you probably cannot use this method.
func NewFromPlain(server, user, password string, transport http.RoundTripper) (*Koji, error) {
	// Create a temporary xmlrpc client.
	// The API doesn't require sessionID, sessionKey and callnum yet,
	// so there's no need to use the custom Koji RoundTripper,
	// let's just use the one that the called passed in.
	rhTransport := CreateRetryableTransport()
	loginClient, err := xmlrpc.NewClient(server, rhTransport)
	if err != nil {
		return nil, err
	}

	args := []interface{}{user, password}
	var reply loginReply
	err = loginClient.Call("login", args, &reply)
	if err != nil {
		return nil, err
	}

	return newKoji(server, transport, reply)
}

// NewFromGSSAPI creates a new Koji session authenticated using GSSAPI.
// Principal and keytab used for the session is passed using credentials
// parameter.
func NewFromGSSAPI(server string, credentials *GSSAPICredentials, transport http.RoundTripper) (*Koji, error) {
	// Create a temporary xmlrpc client with kerberos transport.
	// The API doesn't require sessionID, sessionKey and callnum yet,
	// so there's no need to use the custom Koji RoundTripper,
	// let's just use the one that the called passed in.
	loginClient, err := xmlrpc.NewClient(server+"/ssllogin", &khttp.Transport{
		KeyTab:    credentials.KeyTab,
		Principal: credentials.Principal,
		Next:      transport,
	})
	if err != nil {
		return nil, err
	}

	var reply loginReply
	err = loginClient.Call("sslLogin", nil, &reply)
	if err != nil {
		return nil, err
	}

	return newKoji(server, transport, reply)
}

// GetAPIVersion gets the version of the API of the remote Koji instance
func (k *Koji) GetAPIVersion() (int, error) {
	var version int
	err := k.xmlrpc.Call("getAPIVersion", nil, &version)
	if err != nil {
		return 0, err
	}

	return version, nil
}

// Logout ends the session
func (k *Koji) Logout() error {
	err := k.xmlrpc.Call("logout", nil, nil)
	if err != nil {
		return err
	}
	return nil
}

// CGInitBuild reserves a build ID and initializes a build
func (k *Koji) CGInitBuild(name, version, release string) (*CGInitBuildResult, error) {
	var buildInfo struct {
		Name    string `xmlrpc:"name"`
		Version string `xmlrpc:"version"`
		Release string `xmlrpc:"release"`
	}

	buildInfo.Name = name
	buildInfo.Version = version
	buildInfo.Release = release

	var result CGInitBuildResult
	err := k.xmlrpc.Call("CGInitBuild", []interface{}{"osbuild", buildInfo}, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

/*
	from `koji/__init__.py`

BUILD_STATES = Enum((

	'BUILDING',
	'COMPLETE',
	'DELETED',
	'FAILED',
	'CANCELED',

))
*/
const (
	_ = iota /* BUILDING */
	_        /* COMPLETED */
	_        /* DELETED */
	buildStateFailed
	buildStateCanceled
)

// CGFailBuild marks an in-progress build as failed
func (k *Koji) CGFailBuild(buildID int, token string) error {
	return k.xmlrpc.Call("CGRefundBuild", []interface{}{"osbuild", buildID, token, buildStateFailed}, nil)
}

// CGCancelBuild marks an in-progress build as cancelled, and
func (k *Koji) CGCancelBuild(buildID int, token string) error {
	return k.xmlrpc.Call("CGRefundBuild", []interface{}{"osbuild", buildID, token, buildStateCanceled}, nil)
}

// CGImport imports previously uploaded content, by specifying its metadata, and the temporary
// directory where it is located.
func (k *Koji) CGImport(build Build, buildRoots []BuildRoot, outputs []BuildOutput, directory, token string) (*CGImportResult, error) {
	m := &Metadata{
		Build:      build,
		BuildRoots: buildRoots,
		Outputs:    outputs,
	}
	metadata, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	const retryCount = 10
	const retryDelay = time.Second

	for attempt := 0; attempt < retryCount; attempt += 1 {
		var result CGImportResult
		err = k.xmlrpc.Call("CGImport", []interface{}{string(metadata), directory, token}, &result)

		if err != nil {
			// Retry when the error mentions a corrupted upload. It's usually
			// just because of NFS inconsistency when the kojihub has multiple
			// replicas.
			if strings.Contains(err.Error(), "Corrupted upload") {
				time.Sleep(retryDelay)
				continue
			}

			// Fail immediately on other errors, they are probably legitimate
			return nil, err
		}

		logrus.Infof("CGImport succeeded after %d attempts", attempt+1)

		return &result, nil
	}

	return nil, fmt.Errorf("failed to import a build after %d attempts: %w", retryCount, err)
}

// uploadChunk uploads a byte slice to a given filepath/filname at a given offset
func (k *Koji) uploadChunk(chunk []byte, filepath, filename string, offset uint64) error {
	// We have to open-code a bastardized version of XML-RPC: We send an octet-stream, as
	// if it was an RPC call, and get a regular XML-RPC reply back. In addition to the
	// standard URL parameters, we also have to pass any other parameters as part of the
	// URL, as the body can only contain the payload.
	u, err := url.Parse(k.server)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Add("filepath", filepath)
	q.Add("filename", filename)
	q.Add("offset", fmt.Sprintf("%v", offset))
	q.Add("fileverify", "adler32")
	q.Add("overwrite", "true")
	u.RawQuery = q.Encode()

	client := createCustomRetryableClient()

	client.HTTPClient = &http.Client{
		Transport: k.transport,
	}

	respData, err := client.Post(u.String(), "application/octet-stream", bytes.NewBuffer(chunk))

	if err != nil {
		return err
	}

	defer respData.Body.Close()

	body, err := io.ReadAll(respData.Body)
	if err != nil {
		return err
	}

	var reply struct {
		Size      int    `xmlrpc:"size"`
		HexDigest string `xmlrpc:"hexdigest"`
	}

	resp := xmlrpc.Response(body)

	if resp.Err() != nil {
		return fmt.Errorf("xmlrpc server returned an error: %v", resp.Err())
	}

	err = resp.Unmarshal(&reply)
	if err != nil {
		return fmt.Errorf("cannot unmarshal the xmlrpc response: %v", err)
	}

	if reply.Size != len(chunk) {
		return fmt.Errorf("Sent a chunk of %d bytes, but server got %d bytes", len(chunk), reply.Size)
	}

	digest := fmt.Sprintf("%08x", adler32.Checksum(chunk))
	if reply.HexDigest != digest {
		return fmt.Errorf("Sent a chunk with Adler32 digest %s, but server computed digest %s", digest, reply.HexDigest)
	}

	return nil
}

// Upload uploads file to the temporary filepath on the kojiserver under the name filename
// The md5sum and size of the file is returned on success.
func (k *Koji) Upload(file io.Reader, filepath, filename string) (string, uint64, error) {
	chunk := make([]byte, 1024*1024) // upload a megabyte at a time
	offset := uint64(0)
	// Koji uses MD5 hashes
	/* #nosec G401 */
	hash := md5.New()
	for {
		n, err := file.Read(chunk)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", 0, err
		}
		err = k.uploadChunk(chunk[:n], filepath, filename, offset)
		if err != nil {
			return "", 0, err
		}

		offset += uint64(n)

		m, err := hash.Write(chunk[:n])
		if err != nil {
			return "", 0, err
		}
		if m != n {
			return "", 0, fmt.Errorf("sent %d bytes, but hashed %d", n, m)
		}
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), offset, nil
}

type Transport struct {
	sessionID  int64
	sessionKey string
	callnum    int
	transport  http.RoundTripper
}

// RoundTrip implements the RoundTripper interface, using the default
// transport. When a session has been established, also pass along the
// session credentials. This may not be how the RoundTripper interface
// is meant to be used, but the underlying XML-RPC helpers don't allow
// us to adjust the URL per-call (these arguments should really be in
// the body).
func (rt *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request, so as not to alter the passed in value.
	rClone := new(http.Request)
	*rClone = *req
	rClone.Header = make(http.Header, len(req.Header))
	for idx, header := range req.Header {
		rClone.Header[idx] = append([]string(nil), header...)
	}

	values := rClone.URL.Query()
	values.Add("session-id", fmt.Sprintf("%v", rt.sessionID))
	values.Add("session-key", rt.sessionKey)
	values.Add("callnum", fmt.Sprintf("%v", rt.callnum))
	rClone.URL.RawQuery = values.Encode()

	// Each call is given a unique callnum.
	rt.callnum++

	return rt.transport.RoundTrip(rClone)
}

func GSSAPICredentialsFromEnv() (*GSSAPICredentials, error) {
	principal, principalExists := os.LookupEnv("OSBUILD_COMPOSER_KOJI_PRINCIPAL")
	keyTab, keyTabExists := os.LookupEnv("OSBUILD_COMPOSER_KOJI_KEYTAB")

	if !principalExists || !keyTabExists {
		return nil, errors.New("Both OSBUILD_COMPOSER_KOJI_PRINCIPAL and OSBUILD_COMPOSER_KOJI_KEYTAB must be set")
	}

	return &GSSAPICredentials{
		Principal: principal,
		KeyTab:    keyTab,
	}, nil
}

func CreateKojiTransport(relaxTimeout time.Duration) http.RoundTripper {
	// Koji for some reason needs TLS renegotiation enabled.
	// Clone the default http rt and enable renegotiation.
	rt := CreateRetryableTransport()

	transport := rt.Client.HTTPClient.Transport.(*http.Transport)

	transport.TLSClientConfig = &tls.Config{
		Renegotiation: tls.RenegotiateOnceAsClient,
		MinVersion:    tls.VersionTLS12,
	}

	// Relax timeouts a bit
	if relaxTimeout > 0 {
		transport.TLSHandshakeTimeout *= relaxTimeout
		transport.DialContext = (&net.Dialer{
			Timeout:   30 * time.Second * relaxTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext
	}

	return rt
}

func customCheckRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	shouldRetry, retErr := rh.DefaultRetryPolicy(ctx, resp, err)

	// DefaultRetryPolicy denies retrying for any certificate related error.
	// Override it in case the error is a timeout.
	if !shouldRetry && err != nil {
		if v, ok := err.(*url.Error); ok {
			if _, ok := v.Err.(x509.UnknownAuthorityError); ok {
				// retry if it's a timeout
				return strings.Contains(strings.ToLower(v.Error()), "timeout"), v
			}
		}
	}

	if !shouldRetry && !(resp.StatusCode >= 200 && resp.StatusCode < 300) {
		logrus.Info("Not retrying: ", resp.Status)
	}

	return shouldRetry, retErr
}

func createCustomRetryableClient() *rh.Client {
	client := rh.NewClient()
	client.Logger = rh.LeveledLogger(&LeveledLogrus{logrus.StandardLogger()})
	client.CheckRetry = customCheckRetry
	return client
}

func CreateRetryableTransport() *rh.RoundTripper {
	rt := rh.RoundTripper{}
	rt.Client = createCustomRetryableClient()
	return &rt
}
