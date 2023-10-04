package exception

import "fmt"

type SignatureDoesNotExistException struct {
    Context string
    Err     error
}

func NewSignatureDoesNotExistException(message string) *SignatureDoesNotExistException {
    return &SignatureDoesNotExistException{
        Context: message,
        Err:     nil,
    }
}
func NewSignatureDoesNotExistExceptionWithErr(message string, err error) *SignatureDoesNotExistException {
    return &SignatureDoesNotExistException{
        Context: message,
        Err:     err,
    }
}

func (e SignatureDoesNotExistException) Error() string {
    return fmt.Sprintf("SignatureDoesNotExistException: %s: %v", e.Context, e.Err)
}
