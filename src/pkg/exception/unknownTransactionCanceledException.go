package exception

import "fmt"

type UnknownTransactionCanceledException struct {
    Context string
    Err     *error
}

func NewUnknownTransactionCanceledException(message string, err error) *UnknownTransactionCanceledException {
    return &UnknownTransactionCanceledException{
        Context: message,
        Err:     &err,
    }
}

func (e *UnknownTransactionCanceledException) Error() string {
    return fmt.Sprintf("UnknownTransactionCanceledException: %s: %v", e.Context, e.Err)
}
