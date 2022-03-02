package test

// RequestBody is an abstract interface for defining request bodies for APICall
type RequestBody interface {
	// Body returns the intended request body as a slice of bytes
	Body() []byte

	// ContentType returns value for Content-Type request header
	ContentType() string
}

// JSONRequestBody is just a simple wrapper over plain string.
//
// Body is just the string converted to a slice of bytes and content type is set to application/json
type JSONRequestBody string

func (b JSONRequestBody) Body() []byte {
	return []byte(b)
}

func (b JSONRequestBody) ContentType() string {
	return "application/json"
}
