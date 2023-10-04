package exception

import "fmt"

type KeywordConditionNotMetException struct {
    Context string
    Err     *error
}

func NewKeywordConditionNotMetExceptionWithError(message string, err error) *KeywordConditionNotMetException {
    return &KeywordConditionNotMetException{
        Context: message,
        Err:     &err,
    }
}

func NewKeywordConditionNotMetException(message string) *KeywordConditionNotMetException {
    return &KeywordConditionNotMetException{
        Context: message,
    }
}
func (e *KeywordConditionNotMetException) Error() string {
    return fmt.Sprintf("KeywordConditionNotMetException: %s: %v", e.Context, e.Err)
}
