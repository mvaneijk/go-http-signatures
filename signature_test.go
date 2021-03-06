package httpsignatures

import (
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/url"
	"testing"
)

const (
	testDate = "Thu, 05 Jan 2012 21:31:40 GMT"
)

// Signing
// Test Signature String Config Parser
func TestConfigParserMissingAlgorithmShouldFail(t *testing.T) {
	var s SignatureParameters
	err := s.FromConfig("Test", "", nil)
	assert.EqualError(t, err, ErrorNoAlgorithmConfigured)
	httpErr, _ := ErrorToHTTPCode(err.Error())
	assert.Equal(t, http.StatusInternalServerError, httpErr)
}

func TestConfigParserMissingKeyIdShouldFail(t *testing.T) {
	var s SignatureParameters
	err := s.FromConfig("", "hmac-sha256", nil)
	assert.EqualError(t, err, ErrorNoKeyIDConfigured)
	httpErr, _ := ErrorToHTTPCode(err.Error())
	assert.Equal(t, http.StatusInternalServerError, httpErr)
}

func TestConfigParserNotRequiredDateHeader(t *testing.T) {
	var s SignatureParameters
	err := s.FromConfig("Test", "hmac-sha256", []string{"(request-target)", "host"})
	assert.Nil(t, err) // It's okay to not require the date header for the signature
	sigParam := SignatureParameters{KeyID: "Test", Algorithm: algorithmHmacSha256, Headers: HeaderValues{},
		HeaderList: []string{"(request-target)", "host"}}
	assert.Equal(t, sigParam, s)
}

func TestConfigParserMissingDateHeader(t *testing.T) {
	var s SignatureParameters
	err := s.FromConfig("Test", "hmac-sha256", nil) // the date header will be implicitly required
	assert.Nil(t, err)

	sigParam := SignatureParameters{KeyID: "Test", Algorithm: algorithmHmacSha256, Headers: HeaderValues{},
		HeaderList: []string{"date"}}
	assert.Equal(t, sigParam, s)

	u, err := url.Parse("https://www.example.com/foo?param=value&pet=dog")
	assert.Nil(t, err)

	r := &http.Request{
		Header: http.Header{
			"Authorization": []string{DefaultTestAuthHeader},
		},
		Method: http.MethodPost,
		URL:    u,
	}
	err = s.ParseRequest(r) // it is not okay to have no date header when required
	assert.EqualError(t, err, ErrorMissingRequiredHeader+" 'date'")
	httpErr, _ := ErrorToHTTPCode(err.Error())
	assert.Equal(t, http.StatusBadRequest, httpErr)
}

// Verification
// Test Signature String From Request Parser
func TestRequestParserMissingSignatureShouldFail(t *testing.T) {
	const authHeader string = `keyId="Test",algorithm="hmac-sha256"`
	u, err := url.Parse("https://www.example.com/foo?param=value&pet=dog")
	assert.Nil(t, err)
	r := &http.Request{
		Header: http.Header{
			"Date":          []string{testDate},
			"Authorization": []string{authHeader},
		},
		Method: http.MethodPost,
		URL:    u,
	}

	var s SignatureParameters
	err = s.FromRequest(r)
	assert.EqualError(t, err, ErrorMissingSignatureParameterSignature)
	httpErr, _ := ErrorToHTTPCode(err.Error())
	assert.Equal(t, http.StatusBadRequest, httpErr)
}

func TestRequestParserMissingAlgorithmShouldFail(t *testing.T) {
	const authHeader string = `keyId="Test",signature="fffff"`
	u, err := url.Parse("https://www.example.com/foo?param=value&pet=dog")
	assert.Nil(t, err)
	r := &http.Request{
		Header: http.Header{
			"Date":          []string{testDate},
			"Authorization": []string{authHeader},
		},
		Method: http.MethodPost,
		URL:    u,
	}

	var s SignatureParameters
	err = s.FromRequest(r)
	assert.EqualError(t, err, ErrorMissingSignatureParameterAlgorithm)
	httpErr, _ := ErrorToHTTPCode(err.Error())
	assert.Equal(t, http.StatusBadRequest, httpErr)
}

func TestRequestParserMissingKeyIdShouldFail(t *testing.T) {
	const authHeader string = `algorithm="hmac-sha256",signature="fffff"`
	u, err := url.Parse("https://www.example.com/foo?param=value&pet=dog")
	assert.Nil(t, err)
	r := &http.Request{
		Header: http.Header{
			"Date":          []string{testDate},
			"Authorization": []string{authHeader},
		},
		Method: http.MethodPost,
		URL:    u,
	}

	var s SignatureParameters
	err = s.FromRequest(r)
	assert.EqualError(t, err, ErrorMissingSignatureParameterKeyId)
	httpErr, _ := ErrorToHTTPCode(err.Error())
	assert.Equal(t, http.StatusBadRequest, httpErr)
}

func TestRequestParserDualHeaderShouldPickLastOne(t *testing.T) {
	const authHeader string = `keyId="Test",algorithm="hmac-sha256",signature="fffff",signature="abcde"`
	u, err := url.Parse("https://www.example.com/foo?param=value&pet=dog")
	assert.Nil(t, err)
	r := &http.Request{
		Header: http.Header{
			"Date":          []string{testDate},
			"Authorization": []string{authHeader},
		},
		Method: http.MethodPost,
		URL:    u,
	}

	var s SignatureParameters
	err = s.FromRequest(r)
	assert.Nil(t, err)
	sigParam := SignatureParameters{KeyID: "Test", Algorithm: algorithmHmacSha256, HeaderList: []string{"date"},
		Headers: HeaderValues{"date": testDate}, Signature: "abcde"}
	assert.Equal(t, sigParam, s)
}

func TestRequestParserMissingDateHeader(t *testing.T) {
	const authHeader string = `keyId="Test",algorithm="hmac-sha256",signature="fffff",headers="(request-target) host"`
	u, err := url.Parse("https://www.example.com/foo?param=value&pet=dog#bar")
	assert.Nil(t, err)
	r := &http.Request{
		Header: http.Header{
			"Date":          []string{testDate},
			"Authorization": []string{authHeader},
		},
		Method: http.MethodPost,
		Host:   "example.com",
		URL:    u,
	}

	var s SignatureParameters
	err = s.FromRequest(r)
	assert.Nil(t, err)
	sigParam := SignatureParameters{KeyID: "Test", Algorithm: algorithmHmacSha256,
		Headers:   HeaderValues{"(request-target)": "post /foo?param=value&pet=dog#bar", "host": "example.com"},
		Signature: "fffff", HeaderList: []string{"(request-target)", "host"}}
	assert.Equal(t, sigParam, s)
}

func TestRequestParserTestURLVersions(t *testing.T) {
	const authHeader string = `keyId="Test",algorithm="hmac-sha256",signature="fffff",headers="(request-target) host"`
	u, err := url.Parse("https://www.example.com/foo?param=value&pet=dog#bar")
	assert.Nil(t, err)
	r := &http.Request{
		Header: http.Header{
			"Date":          []string{testDate},
			"Authorization": []string{authHeader},
		},
		Method: http.MethodPost,
		Host:   "example.com",
		URL:    u,
	}

	var s SignatureParameters
	err = s.FromRequest(r)
	assert.Nil(t, err)
	sigParam := SignatureParameters{KeyID: "Test", Algorithm: algorithmHmacSha256,
		Headers:   HeaderValues{"(request-target)": "post /foo?param=value&pet=dog#bar", "host": "example.com"},
		Signature: "fffff", HeaderList: []string{"(request-target)", "host"}}
	assert.Equal(t, sigParam, s)

	r.URL, err = url.Parse("https://www.example.com/foo?param=value&pet=dog")
	assert.Nil(t, err)
	err = s.FromRequest(r)
	assert.Nil(t, err)
	sigParam = SignatureParameters{KeyID: "Test", Algorithm: algorithmHmacSha256,
		Headers:   HeaderValues{"(request-target)": "post /foo?param=value&pet=dog", "host": "example.com"},
		Signature: "fffff", HeaderList: []string{"(request-target)", "host"}}
	assert.Equal(t, sigParam, s)

	r.URL, err = url.Parse("https://www.example.com/foo?param=value#bar")
	assert.Nil(t, err)
	err = s.FromRequest(r)
	assert.Nil(t, err)
	sigParam = SignatureParameters{KeyID: "Test", Algorithm: algorithmHmacSha256,
		Headers:   HeaderValues{"(request-target)": "post /foo?param=value#bar", "host": "example.com"},
		Signature: "fffff", HeaderList: []string{"(request-target)", "host"}}
	assert.Equal(t, sigParam, s)

	r.URL, err = url.Parse("https://www.example.com/foo")
	assert.Nil(t, err)
	err = s.FromRequest(r)
	assert.Nil(t, err)
	sigParam = SignatureParameters{KeyID: "Test", Algorithm: algorithmHmacSha256,
		Headers:   HeaderValues{"(request-target)": "post /foo", "host": "example.com"},
		Signature: "fffff", HeaderList: []string{"(request-target)", "host"}}
	assert.Equal(t, sigParam, s)
}

func TestRequestParserInvalidKeyShouldBeIgnored(t *testing.T) {
	const authHeader string = `Signature keyId="Test",algorithm="hmac-sha256",
		garbage="bob",signature="fffff"`
	u, err := url.Parse("https://www.example.com/foo?param=value&pet=dog")
	assert.Nil(t, err)
	r := &http.Request{
		Header: http.Header{
			"Date":          []string{testDate},
			"Authorization": []string{authHeader},
		},
		Method: http.MethodPost,
		Host:   "example.com",
		URL:    u,
	}

	var s SignatureParameters
	err = s.FromRequest(r)
	assert.Nil(t, err)
	sigParam := SignatureParameters{KeyID: "Test", Algorithm: algorithmHmacSha256, HeaderList: []string{"date"},
		Headers: HeaderValues{"date": testDate}, Signature: "fffff"}
	assert.Equal(t, sigParam, s)
}

// todo , change hmac back to RSA from example in http-signatures-draft-05
const DefaultTestAuthHeader string = `Signature keyId="Test",algorithm="hmac-sha256",
		signature="ATp0r26dbMIxOopqw0OfABDT7CKMIoENumuruOtarj8n/97Q3htHFYpH8yOSQk3Z5zh8UxUym6FYTb5+
		A0Nz3NRsXJibnYi7brE/4tx5But9kkFGzG+xpUmimN4c3TMN7OFH//+r8hBf7BT9/GmHDUVZT2JzWGLZES2xDOUuMtA="`

func TestRequestParserLoadHeaderMissingDateHeader(t *testing.T) {
	u, err := url.Parse("https://www.example.com/foo?param=value&pet=dog")
	assert.Nil(t, err)
	r := &http.Request{
		Header: http.Header{
			"Authorization": []string{DefaultTestAuthHeader},
		},
		Method: http.MethodPost,
		Host:   "example.com",
		URL:    u,
	}

	var s SignatureParameters
	err = s.FromRequest(r) // the date header will be implicitly required
	assert.EqualError(t, err, ErrorMissingRequiredHeader+" 'date'")
	httpErr, _ := ErrorToHTTPCode(err.Error())
	assert.Equal(t, http.StatusBadRequest, httpErr)
}

// Test Parse SignatureParameters from Request
func TestParseRequestWithNoSignatureShouldFail(t *testing.T) {
	r := &http.Request{
		Header: http.Header{
			"Date": []string{testDate},
		},
	}

	var s SignatureParameters
	err := s.FromRequest(r)
	assert.EqualError(t, err, ErrorNoSignatureHeaderFoundInRequest)
	httpErr, _ := ErrorToHTTPCode(err.Error())
	assert.Equal(t, http.StatusBadRequest, httpErr)
}

func TestParseRequestWithNoHostShouldFail(t *testing.T) {
	r := &http.Request{
		Header: http.Header{
			"Date":          []string{testDate},
			"Authorization": []string{DefaultTestAuthHeader},
		},
		Method: http.MethodPost,
	}

	_, err := requestTargetLine(r)
	assert.EqualError(t, err, ErrorURLNotInRequest)
	httpErr, _ := ErrorToHTTPCode(err.Error())
	assert.Equal(t, http.StatusBadRequest, httpErr)
}

func TestParseRequestWithNoMethodShouldFail(t *testing.T) {
	u, err := url.Parse("https://www.example.com/foo?param=value&pet=dog")
	assert.Nil(t, err)
	r := &http.Request{
		Header: http.Header{
			"Date":          []string{testDate},
			"Authorization": []string{DefaultTestAuthHeader},
		},
		Host: "example.com",
		URL:  u,
	}

	_, err = requestTargetLine(r)
	assert.EqualError(t, err, ErrorMethodNotInRequest)
	httpErr, _ := ErrorToHTTPCode(err.Error())
	assert.Equal(t, http.StatusBadRequest, httpErr)
}
