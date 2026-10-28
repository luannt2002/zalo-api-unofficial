package errs

import (
	"fmt"
	"strings"

	"github.com/luannt2002/zalo-api-unofficial/config"
)

var (
	ErrLoginQRAborted  = NewZCA("login QR aborted by user", "")
	ErrLoginQRDeclined = NewZCA("login QR declined by user", "")

	ErrMissingImageMetadataGetter = NewZCA("missing `imageMetadataGetter`. Please provide it in the Zalo object options.", "")

	ErrFileContentUnavailable     = NewZCA("unable to get file content", "")
	ErrInvalidMessageCount        = NewZCA(fmt.Sprintf("message count out of range (allowed: 1-%d)", config.MaxMessagesPerRequest), "")
	ErrInconsistentGroupRecipient = NewZCA("all messages in a group thread must share the same idTo", "")

	ErrSourceEmpty       = NewZCA("source cannot be empty", "")
	ErrExceedMaxFile     = NewZCA("exceeded maximum number of files per request", "")
	ErrInvalidExtension  = NewZCA("file has an invalid extension", "")
	ErrExceedMaxFileSize = NewZCA("exceeded maximum file size", "")
)

type ZCAError struct {
	Message string
	Op      string
	Cause   error
	Meta    map[string]string
}

// maxCauseLen caps the cause text appended by Error() so one log line cannot grow unbounded.
const maxCauseLen = 512

func (e ZCAError) Error() string {
	base := "ZCAError"

	var s string
	switch {
	case e.Message != "" && e.Op != "":
		s = fmt.Sprintf("%s: %s (%s)", base, e.Message, e.Op)
	case e.Message != "":
		s = fmt.Sprintf("%s: %s", base, e.Message)
	case e.Op != "":
		s = fmt.Sprintf("%s: (%s)", base, e.Op)
	default:
		s = base
	}

	if e.Cause == nil {
		return s
	}
	cause := e.Cause.Error()
	if len(cause) > maxCauseLen {
		cause = strings.ToValidUTF8(cause[:maxCauseLen], "") + "…"
	}
	return s + ": " + cause
}

func (e ZCAError) Unwrap() error { return e.Cause }

func (e ZCAError) Is(target error) bool {
	if target, ok := target.(ZCAError); ok {
		return e.Message == target.Message && e.Op == target.Op
	}
	return false
}

func NewZCA(msg, op string) ZCAError { return ZCAError{Message: msg, Op: op} }

func WrapZCA(msg, op string, cause error) ZCAError {
	return ZCAError{Message: msg, Op: op, Cause: cause}
}
