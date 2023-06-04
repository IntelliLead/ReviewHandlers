package exception

import "fmt"

type ReviewDoesNotExistException struct {
    Context string
    Err     error
}

func NewReviewDoesNotExistException(message string) *ReviewDoesNotExistException {
    return &ReviewDoesNotExistException{
        Context: message,
        Err:     nil,
    }
}
func NewReviewDoesNotExistExceptionWithErr(message string, err error) *ReviewDoesNotExistException {
    return &ReviewDoesNotExistException{
        Context: message,
        Err:     err,
    }
}

func (e ReviewDoesNotExistException) Error() string {
    return fmt.Sprintf("ReviewDoesNotExistException: %s: %v", e.Context, e.Err)
}
