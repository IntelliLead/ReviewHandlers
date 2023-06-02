package exception

import "fmt"

type ReviewAlreadyExistException struct {
    Context string
    Err     error
}

func NewReviewAlreadyExistException(message string, err error) *ReviewAlreadyExistException {
    return &ReviewAlreadyExistException{
        Context: message,
        Err:     err,
    }
}

func (e ReviewAlreadyExistException) Error() string {
    return fmt.Sprintf("ReviewAlreadyExistException: %s: %v", e.Context, e.Err)
}
