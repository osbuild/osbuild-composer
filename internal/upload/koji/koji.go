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
	"fmt"
	"hash/adler32"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	rh "github.com/hashicorp/go-retryablehttp"
	"github.com/kolo/xmlrpc"
	"github.com/ubccr/kerby/khttp"
)

type Koji struct {
	xmlrpc    *xmlrpc.Client
	server    string
	transport http.RoundTripper
	logger    rh.LeveledLogger
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

func newKoji(server string, transport http.RoundTripper, reply loginReply, logger rh.LeveledLogger) (*Koji, error) {
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
		logger:    logger,
	}, nil
}

// NewFromGSSAPI creates a new Koji session authenticated using GSSAPI.
// Principal and keytab used for the session is passed using credentials
// parameter.
func NewFromGSSAPI(
	server string,
	credentials *GSSAPICredentials,
	transport http.RoundTripper,
	logger rh.LeveledLogger) (*Koji, error) {
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

	return newKoji(server, transport, reply, logger)
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

		if k.logger != nil {
			k.logger.Info(fmt.Sprintf("CGImport succeeded after %d attempts", attempt+1))
		}

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

	client := createCustomRetryableClient(k.logger)

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

func CreateKojiTransport(relaxTimeout time.Duration, logger rh.LeveledLogger) http.RoundTripper {
	// Koji for some reason needs TLS renegotiation enabled.
	// Clone the default http rt and enable renegotiation.
	rt := CreateRetryableTransport(logger)

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

func createCustomRetryableClient(logger rh.LeveledLogger) *rh.Client {
	client := rh.NewClient()
	client.Logger = logger

	client.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
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

		if logger != nil && (!shouldRetry && !(resp.StatusCode >= 200 && resp.StatusCode < 300)) {
			logger.Info(fmt.Sprintf("Not retrying: %v", resp.Status))
		}

		return shouldRetry, retErr
	}
	return client
}

func CreateRetryableTransport(logger rh.LeveledLogger) *rh.RoundTripper {
	rt := rh.RoundTripper{}
	rt.Client = createCustomRetryableClient(logger)
	return &rt
}
