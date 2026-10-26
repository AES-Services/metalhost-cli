package command

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protowire"
)

// The API's error contract: every error carries a google.rpc.ErrorInfo detail
// (domain metalhost.net) with a stable reason (aes.errors.v1.ErrorReason without
// its prefix) and metadata request_id, which support uses to find the request.

const errorDomain = "metalhost.net"

type errorInfo struct {
	Reason    string
	RequestID string
}

// reasonedError is a connect error decoded from a JSON response, with its
// ErrorInfo already extracted.
type reasonedError struct {
	err  *connect.Error
	info errorInfo
}

func (e *reasonedError) Error() string { return e.err.Error() }
func (e *reasonedError) Unwrap() error { return e.err }

// errorInfoOf returns the ErrorInfo of err, if any.
func errorInfoOf(err error) (errorInfo, bool) {
	var re *reasonedError
	if errors.As(err, &re) {
		return re.info, re.info.Reason != "" || re.info.RequestID != ""
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return errorInfo{}, false
	}
	for _, d := range ce.Details() {
		if d.Type() != "google.rpc.ErrorInfo" {
			continue
		}
		if info, ok := decodeErrorInfo(d.Bytes()); ok {
			return info, true
		}
	}
	return errorInfo{}, false
}

// decodeErrorInfo parses google.rpc.ErrorInfo (reason=1, domain=2,
// metadata=3 map<string,string>) without the googleapis protos.
func decodeErrorInfo(b []byte) (errorInfo, bool) {
	var out errorInfo
	domain := ""
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return errorInfo{}, false
		}
		b = b[n:]
		if typ != protowire.BytesType {
			n = protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return errorInfo{}, false
			}
			b = b[n:]
			continue
		}
		v, n := protowire.ConsumeBytes(b)
		if n < 0 {
			return errorInfo{}, false
		}
		b = b[n:]
		switch num {
		case 1:
			out.Reason = string(v)
		case 2:
			domain = string(v)
		case 3:
			key, val := "", ""
			for len(v) > 0 {
				en, et, m := protowire.ConsumeTag(v)
				if m < 0 || et != protowire.BytesType {
					return errorInfo{}, false
				}
				v = v[m:]
				s, m := protowire.ConsumeBytes(v)
				if m < 0 {
					return errorInfo{}, false
				}
				v = v[m:]
				if en == 1 {
					key = string(s)
				} else if en == 2 {
					val = string(s)
				}
			}
			if key == "request_id" {
				out.RequestID = val
			}
		}
	}
	return out, domain == errorDomain
}

// errorInfoFromJSON reads ErrorInfo from a Connect JSON error body, where the
// server includes each detail's decoded form under "debug".
func errorInfoFromJSON(body []byte) errorInfo {
	var payload struct {
		Details []struct {
			Type  string `json:"type"`
			Debug struct {
				Reason   string            `json:"reason"`
				Domain   string            `json:"domain"`
				Metadata map[string]string `json:"metadata"`
			} `json:"debug"`
		} `json:"details"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return errorInfo{}
	}
	for _, d := range payload.Details {
		if d.Type == "google.rpc.ErrorInfo" && d.Debug.Domain == errorDomain {
			return errorInfo{Reason: d.Debug.Reason, RequestID: d.Debug.Metadata["request_id"]}
		}
	}
	return errorInfo{}
}

// newRequestID returns a random RFC 4122 version 4 UUID for retry-safe
// mutations.
func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
