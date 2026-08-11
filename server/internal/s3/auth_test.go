package s3

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// signRequest builds a valid AWS SigV4 Authorization header for the request
// using the given secret, amzDate timestamp (20060102T150405Z), region, and
// service. It reuses the same algorithm as VerifySigV4 so the happy path test
// exercises the full round trip (including the Host header that clients sign).
func signRequest(r *http.Request, secret, amzDate, region, service string) {
	dateStamp := amzDate[:8]
	r.Header.Set("X-Amz-Date", amzDate)

	payloadHash := sha256Hex(nil)
	r.Header.Set("X-Amz-Content-Sha256", payloadHash)

	host := r.Host
	if host == "" {
		host = r.URL.Host
	}

	canonicalHeaders := "host:" + host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"

	uri := r.URL.EscapedPath()
	if uri == "" {
		uri = "/"
	}
	canonicalRequest := r.Method + "\n" + uri + "\n" + r.URL.RawQuery + "\n" +
		canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash

	scope := dateStamp + "/" + region + "/" + service + "/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))

	signingKey := hmacSHA256([]byte("AWS4"+secret), dateStamp)
	signingKey = hmacSHA256(signingKey, region)
	signingKey = hmacSHA256(signingKey, service)
	signingKey = hmacSHA256(signingKey, "aws4_request")

	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	r.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=test/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestVerifySigV4(t *testing.T) {
	const secret = "8b0cc094e3442cc5d46cac82d48e18e350bf982f"
	const region = "us-east-1"
	const service = "s3"

	newRequest := func() *http.Request {
		req := httptest.NewRequest("GET", "http://localhost:8081/api/s3/org-1/hello.txt", nil)
		return req
	}

	t.Run("valid signature", func(t *testing.T) {
		req := newRequest()
		signRequest(req, secret, time.Now().UTC().Format("20060102T150405Z"), region, service)
		if err := VerifySigV4(req, secret); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("wrong secret", func(t *testing.T) {
		req := newRequest()
		signRequest(req, secret, time.Now().UTC().Format("20060102T150405Z"), region, service)
		if err := VerifySigV4(req, "wrong-secret"); err == nil || !strings.Contains(err.Error(), "signature mismatch") {
			t.Fatalf("expected signature mismatch, got %v", err)
		}
	})

	t.Run("expired timestamp", func(t *testing.T) {
		req := newRequest()
		signRequest(req, secret, time.Now().UTC().Add(-time.Hour).Format("20060102T150405Z"), region, service)
		if err := VerifySigV4(req, secret); err == nil || !strings.Contains(err.Error(), "timestamp out of range") {
			t.Fatalf("expected timestamp out of range, got %v", err)
		}
	})

	t.Run("unsupported scheme", func(t *testing.T) {
		req := newRequest()
		req.Header.Set("Authorization", "Bearer token")
		if err := VerifySigV4(req, secret); err == nil || !strings.Contains(err.Error(), "unsupported authorization scheme") {
			t.Fatalf("expected unsupported scheme, got %v", err)
		}
	})

	t.Run("incomplete header", func(t *testing.T) {
		req := newRequest()
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=test")
		if err := VerifySigV4(req, secret); err == nil || !strings.Contains(err.Error(), "incomplete authorization header") {
			t.Fatalf("expected incomplete header, got %v", err)
		}
	})

	t.Run("invalid credential terminator", func(t *testing.T) {
		req := newRequest()
		req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=test/20260101/us-east-1/s3/not-aws4_request, SignedHeaders=host, Signature=abc")
		if err := VerifySigV4(req, secret); err == nil || !strings.Contains(err.Error(), "invalid credential terminator") {
			t.Fatalf("expected invalid credential terminator, got %v", err)
		}
	})

	t.Run("missing x-amz-date", func(t *testing.T) {
		req := newRequest()
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=test/20260101/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc")
		if err := VerifySigV4(req, secret); err == nil || !strings.Contains(err.Error(), "missing x-amz-date") {
			t.Fatalf("expected missing x-amz-date, got %v", err)
		}
	})
}
