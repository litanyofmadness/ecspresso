package ecspresso

import (
	"errors"

	"github.com/aws/smithy-go"
)

type ErrSkipVerify string

func (e ErrSkipVerify) Error() string {
	return string(e)
}

type ErrNotFound string

func (e ErrNotFound) Error() string {
	return string(e)
}

type ErrConflictOptions string

func (e ErrConflictOptions) Error() string {
	return string(e)
}

type ErrPermissionDenied string

func (e ErrPermissionDenied) Error() string {
	return string(e)
}

var (
	errNotFound         = ErrNotFound("not found")
	errSkipVerify       = ErrSkipVerify("skip verify")
	errPermissionDenied = ErrPermissionDenied("permission denied")
)

func isPermissionError(err error) bool {
	// Check if it's wrapped in OperationError
	var oe *smithy.OperationError
	if errors.As(err, &oe) {
		err = oe.Err
	}

	// Check the actual API error
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "AccessDeniedException", "UnauthorizedException",
			"Forbidden", "AccessDenied", "InvalidUserID.NotFound":
			return true
		}
	}
	return false
}

func wrapPermissionError(err error) error {
	if err == nil {
		return nil
	}
	if isPermissionError(err) {
		return ErrPermissionDenied(err.Error())
	}
	return err
}
