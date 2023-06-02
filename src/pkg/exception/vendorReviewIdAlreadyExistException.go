package exception

import "fmt"

type VendorReviewIdAlreadyExistException struct {
    Context string
    Err     error
}

func NewVendorReviewIdAlreadyExistException(message string, err error) *VendorReviewIdAlreadyExistException {
    return &VendorReviewIdAlreadyExistException{
        Context: message,
        Err:     err,
    }
}

func (e VendorReviewIdAlreadyExistException) Error() string {
    return fmt.Sprintf("VendorReviewIdAlreadyExistException: %s: %v", e.Context, e.Err)
}
