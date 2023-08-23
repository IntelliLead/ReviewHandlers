package exception

import "fmt"

type BusinessDoesNotExistException struct {
    Context string
    Err     error
}

func NewBusinessDoesNotExistException(message string) *BusinessDoesNotExistException {
    return &BusinessDoesNotExistException{
        Context: message,
        Err:     nil,
    }
}
func NewBusinessDoesNotExistExceptionWithErr(message string, err error) *BusinessDoesNotExistException {
    return &BusinessDoesNotExistException{
        Context: message,
        Err:     err,
    }
}

func (e BusinessDoesNotExistException) Error() string {
    return fmt.Sprintf("BusinessDoesNotExistException: %s: %v", e.Context, e.Err)
}
