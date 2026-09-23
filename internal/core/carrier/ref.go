package carrier

import (
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"
)

var idPattern = regexp.MustCompile(`^(dec|spec|note|prob|ev|opt)-([0-9]{8})-[0-9a-f]{8}$`)
var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var localIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

type Ref struct {
	RecordID string `json:"record_id,omitempty"`
	Alias    string `json:"alias,omitempty"`
	Digest   string `json:"digest,omitempty"`
	ClaimID  string `json:"claim_id,omitempty"`
}

func (r Ref) Pinned() bool { return r.Digest != "" }
func (r Ref) String() string {
	s := r.RecordID
	if r.Alias != "" {
		s = "spec:" + r.Alias
	}
	if r.Digest != "" {
		s += "@" + r.Digest
	}
	if r.ClaimID != "" {
		s += "#" + r.ClaimID
	}
	return s
}
func ValidDigest(s string) bool { return digestPattern.MatchString(s) }
func ValidID(s string) bool {
	m := idPattern.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	_, err := time.Parse("20060102", m[2])
	return err == nil
}
func ParseRef(s string) (Ref, error) {
	var r Ref
	base, claim, hasClaim := strings.Cut(s, "#")
	if hasClaim {
		if !localIDPattern.MatchString(claim) {
			return r, fmt.Errorf("invalid claim/use ID in %q", s)
		}
		r.ClaimID = claim
	}
	id, digest, pinned := strings.Cut(base, "@")
	if pinned {
		if !ValidDigest(digest) || !ValidID(id) {
			return Ref{}, fmt.Errorf("pinned ref requires a record ID and full SHA-256: %q", s)
		}
		r.Digest = digest
	}
	if strings.HasPrefix(id, "spec:") && !pinned {
		r.Alias = strings.TrimPrefix(id, "spec:")
		if !slugPattern.MatchString(r.Alias) {
			return Ref{}, fmt.Errorf("invalid spec alias %q", id)
		}
	} else if ValidID(id) {
		r.RecordID = id
	} else {
		return Ref{}, fmt.Errorf("invalid record ref %q", s)
	}
	return r, nil
}

// ValidateSelector validates local syntax and root containment, not existence.
func ValidateSelector(s string) error {
	kind, value, ok := strings.Cut(s, ":")
	if !ok || value == "" {
		return fmt.Errorf("selector requires kind and value")
	}
	switch kind {
	case "file", "dir", "sym":
		p := value
		if kind == "sym" {
			var name string
			p, name, ok = strings.Cut(value, "::")
			if !ok || strings.TrimSpace(name) == "" {
				return fmt.Errorf("symbol selector requires path::name")
			}
		}
		if strings.ContainsAny(p, "\\\x00\r\n") || strings.HasPrefix(p, "/") || p == "" {
			return fmt.Errorf("path must be project-relative")
		}
		clean := path.Clean(p)
		if clean == ".." || strings.HasPrefix(clean, "../") || clean != p {
			return fmt.Errorf("path must be normalized within project root")
		}
		return nil
	default:
		return fmt.Errorf("unsupported implementation selector %q", kind)
	}
}
func validAbout(s string) bool {
	if _, err := ParseRef(s); err == nil {
		return true
	}
	if ValidateSelector(s) == nil {
		return true
	}
	kind, value, ok := strings.Cut(s, ":")
	return ok && (kind == "system" || kind == "domain") && strings.TrimSpace(value) != "" && !strings.ContainsAny(value, "\x00\r\n")
}
