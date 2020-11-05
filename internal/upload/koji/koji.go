package koji

import (
	"bytes"
	// Koji uses MD5 hashes
	// #gosec G501
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"hash/adler32"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/kolo/xmlrpc"
	"github.com/ubccr/kerby/khttp"
)

type Koji struct {
	xmlrpc    *xmlrpc.Client
	server    string
	transport http.RoundTripper
}

type TypeInfo struct {
	Image struct{} `json:"image"`
}

type ImageBuildExtra struct {
	TypeInfo TypeInfo `json:"typeinfo"`
}

type ImageBuild struct {
	BuildID   uint64          `json:"build_id"`
	TaskID    uint64          `json:"task_id"`
	Name      string          `json:"name"`
	Version   string          `json:"version"`
	Release   string          `json:"release"`
	Source    string          `json:"source"`
	StartTime int64           `json:"start_time"`
	EndTime   int64           `json:"end_time"`
	Extra     ImageBuildExtra `json:"extra"`
}

type Host struct {
	Os   string `json:"os"`
	Arch string `json:"arch"`
}

type ContentGenerator struct {
	Name    string `json:"name"` // Must be 'osbuild'.
	Version string `json:"version"`
}

type Container struct {
	Type string `json:"type"`
	Arch string `json:"arch"`
}

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type RPM struct {
	Type      string  `json:"type"` // must be 'rpm'
	Name      string  `json:"name"`
	Version   string  `json:"version"`
	Release   string  `json:"release"`
	Epoch     *string `json:"epoch,omitempty"`
	Arch      string  `json:"arch"`
	Sigmd5    string  `json:"sigmd5"`
	Signature *string `json:"signature"`
}

type BuildRoot struct {
	ID               uint64           `json:"id"`
	Host             Host             `json:"host"`
	ContentGenerator ContentGenerator `json:"content_generator"`
	Container        Container        `json:"container"`
	Tools            []Tool           `json:"tools"`
	RPMs             []RPM            `json:"components"`
}

type ImageExtraInfo struct {
	// TODO: Ideally this is where the pipeline would be passed.
	Arch string `json:"arch"` // TODO: why?
}

type ImageExtra struct {
	Info ImageExtraInfo `json:"image"`
}

type Image struct {
	BuildRootID  uint64     `json:"buildroot_id"`
	Filename     string     `json:"filename"`
	FileSize     uint64     `json:"filesize"`
	Arch         string     `json:"arch"`
	ChecksumType string     `json:"checksum_type"` // must be 'md5'
	MD5          string     `json:"checksum"`
	Type         string     `json:"type"`
	RPMs         []RPM      `json:"components"`
	Extra        ImageExtra `json:"extra"`
}

type Metadata struct {
	MetadataVersion int         `json:"metadata_version"` // must be '0'
	ImageBuild      ImageBuild  `json:"build"`
	BuildRoots      []BuildRoot `json:"buildroots"`
	Images          []Image     `json:"output"`
}

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
	loginClient, err := xmlrpc.NewClient(server, http.DefaultTransport)
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

/* from `koji/__init__.py`
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
func (k *Koji) CGImport(build ImageBuild, buildRoots []BuildRoot, images []Image, directory, token string) (*CGImportResult, error) {
	m := &Metadata{
		ImageBuild: build,
		BuildRoots: buildRoots,
		Images:     images,
	}
	metadata, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	var result CGImportResult
	err = k.xmlrpc.Call("CGImport", []interface{}{string(metadata), directory, token}, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
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
	u.RawQuery = q.Encode()

	client := http.Client{Transport: k.transport}
	respData, err := client.Post(u.String(), "application/octet-stream", bytes.NewBuffer(chunk))
	if err != nil {
		return err
	}
	defer respData.Body.Close()

	body, err := ioutil.ReadAll(respData.Body)
	if err != nil {
		return err
	}

	var reply struct {
		Size      int    `xmlrpc:"size"`
		HexDigest string `xmlrpc:"hexdigest"`
	}

	err = processXMLRPCResponse(body, &reply)
	if err != nil {
		return err
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
	// #gosec G501
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
