package exception

import "fmt"

type BusinessAlreadyExistException struct {
    Context string
    Err     error
}

func NewBusinessAlreadyExistException(message string, err error) *BusinessAlreadyExistException {
    return &BusinessAlreadyExistException{
        Context: message,
        Err:     err,
    }
}

func (e *BusinessAlreadyExistException) Error() string {
    return fmt.Sprintf("BusinessAlreadyExistException: %s: %v", e.Context, e.Err)
}
