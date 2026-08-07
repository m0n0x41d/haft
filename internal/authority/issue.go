package authority

import (
	"fmt"
	"strings"
	"unicode"
)

const authorityIssueReviewDigestDomain = "haft.authority.issue-review/v1"

func validateAuthorityIssueReviewText(reviewText string) error {
	if reviewText == "" || strings.TrimSpace(reviewText) != reviewText {
		return fmt.Errorf("authority issue review must be non-empty canonical text")
	}
	if len(reviewText) > 64*1024 {
		return fmt.Errorf("authority issue review exceeds 64 KiB")
	}
	invalidControl := strings.ContainsFunc(reviewText, func(value rune) bool {
		return unicode.IsControl(value) && value != '\n' && value != '\t'
	})
	if invalidControl {
		return fmt.Errorf("authority issue review contains unsupported control characters")
	}
	return nil
}
