// Package s3 provides AWS SigV4 signature verification for the S3-compatible
// gateway (M8). Requests to /api/s3/* are signed with the AWS Signature
// Version 4 algorithm using an S3APIKey's access key id + secret access key.
package s3

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// VerifySigV4 validates the AWS SigV4 Authorization header signature for the
// request using the given secret access key. Returns nil when the signature is
// valid and fresh (within ±15 minutes of now to prevent replay).
func VerifySigV4(r *http.Request, secretAccessKey string) error {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		return fmt.Errorf("unsupported authorization scheme")
	}

	parts := map[string]string{}
	for _, kv := range strings.Split(strings.TrimPrefix(auth, "AWS4-HMAC-SHA256 "), ",") {
		kv = strings.TrimSpace(kv)
		idx := strings.Index(kv, "=")
		if idx < 0 {
			continue
		}
		parts[strings.TrimSpace(kv[:idx])] = strings.TrimSpace(kv[idx+1:])
	}
	credential := parts["Credential"]
	signedHeadersStr := parts["SignedHeaders"]
	providedSignature := parts["Signature"]
	if credential == "" || signedHeadersStr == "" || providedSignature == "" {
		return fmt.Errorf("incomplete authorization header")
	}

	credParts := strings.Split(credential, "/")
	if len(credParts) != 5 {
		return fmt.Errorf("invalid credential scope")
	}
	dateStamp, region, service := credParts[1], credParts[2], credParts[3]
	if credParts[4] != "aws4_request" {
		return fmt.Errorf("invalid credential terminator")
	}

	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return fmt.Errorf("missing x-amz-date")
	}
	t, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return fmt.Errorf("invalid x-amz-date: %w", err)
	}
	if d := time.Since(t); d > 15*time.Minute || d < -15*time.Minute {
		return fmt.Errorf("request timestamp out of range")
	}

	// SignedHeaders must be sorted lowercase; the client sends them sorted.
	signedHeaders := strings.Split(signedHeadersStr, ";")
	sort.Strings(signedHeaders)
	signedHeadersStr = strings.Join(signedHeaders, ";")

	// Payload hash: use the header value as-is (the client signs it). For
	// requests without the header (e.g. GET), fall back to the empty payload.
	payloadHash := r.Header.Get("X-Amz-Content-Sha256")
	if payloadHash == "" {
		h := sha256.Sum256(nil)
		payloadHash = hex.EncodeToString(h[:])
	}

	canonicalURI := r.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	var canonicalHeaders strings.Builder
	for _, h := range signedHeaders {
		v := r.Header.Get(h)
		if h == "host" {
			v = r.Host
		}
		canonicalHeaders.WriteString(h + ":" + strings.TrimSpace(v) + "\n")
	}

	canonicalRequest := strings.Join([]string{
		r.Method,
		canonicalURI,
		canonicalQueryString(r.URL.RawQuery),
		canonicalHeaders.String(),
		signedHeadersStr,
		payloadHash,
	}, "\n")

	scope := dateStamp + "/" + region + "/" + service + "/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hexSha256(canonicalRequest)

	kDate := hmacSHA256([]byte("AWS4"+secretAccessKey), dateStamp)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	expected := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	if !hmac.Equal([]byte(expected), []byte(providedSignature)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// AccessKeyID extracts the access key id from the Authorization header's
// Credential field (the first segment, e.g. "AKID/date/region/s3/aws4_request").
func AccessKeyID(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		return "", fmt.Errorf("unsupported authorization scheme")
	}
	for _, kv := range strings.Split(strings.TrimPrefix(auth, "AWS4-HMAC-SHA256 "), ",") {
		kv = strings.TrimSpace(kv)
		if !strings.HasPrefix(kv, "Credential=") {
			continue
		}
		cred := strings.TrimPrefix(kv, "Credential=")
		idx := strings.Index(cred, "/")
		if idx < 0 {
			return "", fmt.Errorf("invalid credential scope")
		}
		return cred[:idx], nil
	}
	return "", fmt.Errorf("missing credential")
}

// canonicalQueryString sorts and URI-encodes the raw query string per SigV4.
func canonicalQueryString(rawQuery string) string {
	if rawQuery == "" {
		return ""
	}
	vals, _ := url.ParseQuery(rawQuery)
	var keys []string
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		vs := vals[k]
		sort.Strings(vs)
		for _, v := range vs {
			parts = append(parts, awsURIEncode(k, true)+"="+awsURIEncode(v, true))
		}
	}
	return strings.Join(parts, "&")
}

// awsURIEncode percent-encodes s per AWS SigV4 rules: every byte except the
// unreserved set A-Za-z0-9-_.~ is encoded. When encodeSlash is false, '/'
// is left as-is (used for path segments — though here we use EscapedPath so
// it is only relevant for query values).
func awsURIEncode(s string, encodeSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUnreserved(c) || (c == '/' && !encodeSlash) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func isUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
		c == '-' || c == '.' || c == '_' || c == '~'
}

func hexSha256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}
