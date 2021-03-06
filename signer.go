package httpsignatures

import (
	"errors"
	"net/http"
	"time"
)

type signer struct {
	algorithm string
	headers   []string
}

// NewSigner adds an algorithm to the signer algorithms
func NewSigner(algorithm string, headers ...string) *signer {
	return &signer{
		algorithm: algorithm,
		headers:   headers,
	}
}

// SignRequest adds a http signature to the Signature: HTTP Header
func (s signer) SignRequest(r *http.Request, keyID string, keyB64 string) error {
	signature, err := s.createHTTPSignatureString(r, keyID, keyB64)
	if err != nil {
		return err
	}

	r.Header.Add("Signature", signature)
	return nil
}

// AuthRequest adds a http signature to the Authorization: HTTP Header
func (s signer) AuthRequest(r *http.Request, keyID string, keyB64 string) error {
	signature, err := s.createHTTPSignatureString(r, keyID, keyB64)
	if err != nil {
		return err
	}

	r.Header.Add("Authorization", "Signature "+signature)
	return nil
}

func (s signer) createHTTPSignatureString(r *http.Request, keyID string, keyB64 string) (string, error) {
	sig := SignatureParameters{}
	if err := sig.FromConfig(keyID, s.algorithm, s.headers); err != nil {
		return "", err
	}

	if err := sig.ParseRequest(r); err != nil {
		return "", err
	}

	signature, err := sig.calculateSignature(keyB64)
	if err != nil {
		return "", err
	}

	return sig.hTTPSignatureString(signature), nil
}

// VerifyRequest verifies the signature added to the request and returns true if it is OK
func VerifyRequest(r *http.Request, keyLookUp func(keyID string) (string, error), allowedClockSkew int,
	allowedAlgorithms []string, requiredHeaders ...string) (bool, error) {

	sig := SignatureParameters{}

	if err := sig.FromRequest(r); err != nil {
		return false, err
	}

	isAlgorithmAllowed := false
	for _, algorithm := range allowedAlgorithms {
		if sig.Algorithm.Name == algorithm {
			isAlgorithmAllowed = true
			break
		}
	}
	if !isAlgorithmAllowed {
		return false, errors.New(ErrorAlgorithmNotAllowed)
	}

	for _, header := range requiredHeaders {
		if sig.Headers[header] == "" {
			return false, errors.New(ErrorRequiredHeaderNotInHeaderList + ": '" + header + "'")
		}
	}

	if allowedClockSkew > -1 {
		if allowedClockSkew == 0 {
			return false, errors.New(ErrorYouProbablyMisconfiguredAllowedClockSkew)
		}
		// check if difference between date and date.Now exceeds allowedClockSkew
		var date string
		// if 'X-Date' header exists, prefer this header above 'Date'
		if d := sig.Headers["x-date"]; len(d) != 0 {
			date = d
		} else if d := sig.Headers["date"]; len(d) != 0 {
			date = d
		} else {
			return false, errors.New(ErrorDateHeaderIsMissingForClockSkewComparison)
		}
		if hdrDate, err := time.Parse(time.RFC1123, date); err == nil {
			if (int)(time.Since(hdrDate).Seconds()) > (allowedClockSkew) {
				return false, errors.New(ErrorAllowedClockskewExceeded)
			}
		} else {
			return false, err
		}
	}
	key, err := keyLookUp(sig.KeyID)
	if err != nil {
		return false, err
	}
	return sig.Verify(key)
}
