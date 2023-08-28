package exception

import "fmt"

type AutoQuickReplyConditionNotMetException struct {
    Context string
    Err     *error
}

func NewAutoQuickReplyConditionNotMetExceptionWithError(message string, err error) *AutoQuickReplyConditionNotMetException {
    return &AutoQuickReplyConditionNotMetException{
        Context: message,
        Err:     &err,
    }
}

func NewAutoQuickReplyConditionNotMetException(message string) *AutoQuickReplyConditionNotMetException {
    return &AutoQuickReplyConditionNotMetException{
        Context: message,
    }
}
func (e *AutoQuickReplyConditionNotMetException) Error() string {
    return fmt.Sprintf("AutoQuickReplyConditionNotMetException: %s: %v", e.Context, e.Err)
}
