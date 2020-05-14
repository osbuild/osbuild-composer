package koji

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"hash/adler32"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/kolo/xmlrpc"
)

type Koji struct {
	sessionID  int64
	sessionKey string
	callnum    int
	xmlrpc     *xmlrpc.Client
	server     string
}

type BuildExtra struct {
	Image interface{} `json:"image"` // No extra info tracked at build level.
}

type Build struct {
	Name      string     `json:"name"`
	Version   string     `json:"version"`
	Release   string     `json:"release"`
	Source    string     `json:"source"`
	StartTime int64      `json:"start_time"`
	EndTime   int64      `json:"end_time"`
	Extra     BuildExtra `json:"extra"`
}

type Host struct {
	Os   string `json:"os"`
	Arch string `json:"arch"`
}

type ContentGenerator struct {
	Name    string `json:"name"`
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

type Component struct {
	Type      string  `json:"type"` // must be 'rpm'
	Name      string  `json:"name"`
	Version   string  `json:"version"`
	Release   string  `json:"release"`
	Epoch     *uint64 `json:"epoch"`
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
	Components       []Component      `json:"components"`
}

type OutputExtraImageInfo struct {
	// TODO: Ideally this is where the pipeline would be passed.
	Arch string `json:"arch"` // TODO: why?
}

type OutputExtra struct {
	Image OutputExtraImageInfo `json:"image"`
}

type Output struct {
	BuildRootID  uint64      `json:"buildroot_id"`
	Filename     string      `json:"filename"`
	FileSize     uint64      `json:"filesize"`
	Arch         string      `json:"arch"`
	ChecksumType string      `json:"checksum_type"` // must be 'md5'
	MD5          string      `json:"checksum"`
	Type         string      `json:"type"`
	Components   []Component `json:"component"`
	Extra        OutputExtra `json:"extra"`
}

type Metadata struct {
	MetadataVersion int         `json:"metadata_version"` // must be '0'
	Build           Build       `json:"build"`
	BuildRoots      []BuildRoot `json:"buildroots"`
	Output          []Output    `json:"output"`
}

// RoundTrip implements the RoundTripper interface, using the default
// transport. When a session has been established, also pass along the
// session credentials. This may not be how the RoundTripper interface
// is meant to be used, but the underlying XML-RPC helpers don't allow
// us to adjust the URL per-call (these arguments should really be in
// the body).
func (k *Koji) RoundTrip(req *http.Request) (*http.Response, error) {
	if k.sessionKey == "" {
		return http.DefaultTransport.RoundTrip(req)
	}

	// Clone the request, so as not to alter the passed in value.
	rClone := new(http.Request)
	*rClone = *req
	rClone.Header = make(http.Header, len(req.Header))
	for idx, header := range req.Header {
		rClone.Header[idx] = append([]string(nil), header...)
	}

	values := rClone.URL.Query()
	values.Add("session-id", fmt.Sprintf("%v", k.sessionID))
	values.Add("session-key", k.sessionKey)
	values.Add("callnum", fmt.Sprintf("%v", k.callnum))
	rClone.URL.RawQuery = values.Encode()

	// Each call is given a unique callnum.
	k.callnum++

	return http.DefaultTransport.RoundTrip(rClone)
}

func New(server string) (*Koji, error) {
	k := &Koji{}
	client, err := xmlrpc.NewClient(server, k)
	if err != nil {
		return nil, err
	}
	k.xmlrpc = client
	k.server = server
	return k, nil
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

// Login sets up a new session with the given user/password
func (k *Koji) Login(user, password string) error {
	args := []interface{}{user, password}
	var reply struct {
		SessionID  int64  `xmlrpc:"session-id"`
		SessionKey string `xmlrpc:"session-key"`
	}
	err := k.xmlrpc.Call("login", args, &reply)
	if err != nil {
		return err
	}
	k.sessionID = reply.SessionID
	k.sessionKey = reply.SessionKey
	k.callnum = 0
	return nil
}

// Logout ends the session
func (k *Koji) Logout() error {
	err := k.xmlrpc.Call("logout", nil, nil)
	if err != nil {
		return err
	}
	return nil
}

// CGImport imports previously uploaded content, by specifying its metadata, and the temporary
// directory where it is located.
func (k *Koji) CGImport(build Build, buildRoots []BuildRoot, output []Output, directory string) error {
	m := &Metadata{
		Build:      build,
		BuildRoots: buildRoots,
		Output:     output,
	}
	metadata, err := json.Marshal(m)
	if err != nil {
		return err
	}

	var result interface{}
	err = k.xmlrpc.Call("CGImport", []interface{}{string(metadata), directory}, &result)
	if err != nil {
		return err
	}

	return nil
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
	q.Add("session-id", fmt.Sprintf("%v", k.sessionID))
	q.Add("session-key", k.sessionKey)
	q.Add("callnum", fmt.Sprintf("%v", k.callnum))
	u.RawQuery = q.Encode()

	// Each call is given a unique callnum.
	k.callnum++

	resp, err := http.Post(u.String(), "application/octet-stream", bytes.NewBuffer(chunk))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	err = xmlrpc.Response.Err(body)
	if err != nil {
		return err
	}

	var reply struct {
		Size      int    `xmlrpc:"size"`
		HexDigest string `xmlrpc:"hexdigest"`
	}

	err = xmlrpc.Response.Unmarshal(body, &reply)
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
