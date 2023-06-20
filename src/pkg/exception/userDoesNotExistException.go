package exception

import "fmt"

type UserDoesNotExistException struct {
    Context string
    Err     error
}

func NewUserDoesNotExistException(message string) *UserDoesNotExistException {
    return &UserDoesNotExistException{
        Context: message,
        Err:     nil,
    }
}
func NewUserDoesNotExistExceptionWithErr(message string, err error) *UserDoesNotExistException {
    return &UserDoesNotExistException{
        Context: message,
        Err:     err,
    }
}

func (e UserDoesNotExistException) Error() string {
    return fmt.Sprintf("UserDoesNotExistException: %s: %v", e.Context, e.Err)
}
