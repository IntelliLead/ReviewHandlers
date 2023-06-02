package exception

import "fmt"

type InvalidReviewException struct {
    Context string
    Err     *error
}

func NewInvalidReviewExceptionWithError(message string, err error) *InvalidReviewException {
    return &InvalidReviewException{
        Context: message,
        Err:     &err,
    }
}

func NewInvalidReviewException(message string) *InvalidReviewException {
    return &InvalidReviewException{
        Context: message,
    }
}
func (e *InvalidReviewException) Error() string {
    return fmt.Sprintf("InvalidReviewException: %s: %v", e.Context, e.Err)
}
