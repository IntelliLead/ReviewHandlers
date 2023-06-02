package exception

import "fmt"

type UnknownDDBException struct {
    Context string
    Err     *error
}

func NewUnknownDDBException(message string, err error) *UnknownDDBException {
    return &UnknownDDBException{
        Context: message,
        Err:     &err,
    }
}

func (e *UnknownDDBException) Error() string {
    return fmt.Sprintf("UnknownDDBException: %s: %v", e.Context, e.Err)
}
