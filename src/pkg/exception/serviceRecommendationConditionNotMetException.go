package exception

import "fmt"

type ServiceRecommendationConditionNotMetException struct {
    Context string
    Err     *error
}

func NewServiceRecommendationConditionNotMetExceptionWithError(message string, err error) *ServiceRecommendationConditionNotMetException {
    return &ServiceRecommendationConditionNotMetException{
        Context: message,
        Err:     &err,
    }
}

func NewServiceRecommendationConditionNotMetException(message string) *ServiceRecommendationConditionNotMetException {
    return &ServiceRecommendationConditionNotMetException{
        Context: message,
    }
}
func (e *ServiceRecommendationConditionNotMetException) Error() string {
    return fmt.Sprintf("ServiceRecommendationConditionNotMetException: %s: %v", e.Context, e.Err)
}
