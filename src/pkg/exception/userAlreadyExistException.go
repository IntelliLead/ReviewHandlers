package exception

import "fmt"

type UserAlreadyExistException struct {
	Context string
	Err     error
}

func NewUserAlreadyExistException(message string, err error) *UserAlreadyExistException {
	return &UserAlreadyExistException{
		Context: message,
		Err:     err,
	}
}

func (e *UserAlreadyExistException) Error() string {
	return fmt.Sprintf("UserAlreadyExistException: %s: %v", e.Context, e.Err)
}
