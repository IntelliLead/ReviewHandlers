package middleware

import (
    "context"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/metric"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/metric/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/aws/aws-lambda-go/events"
)

var (
    _log = logger.NewLogger()
)

func MetricMiddleware(handlerName enum.HandlerName,
    handler func(context.Context, events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error)) func(context.Context, events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    return func(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
        var response events.LambdaFunctionURLResponse
        var err error

        // Use defer to ensure metric emission even in case of panic
        defer func() {
            r := recover()
            if r != nil {
                _log.Infof("Emitting 5XXError metric due to panic")
                metric.EmitLambdaMetric(enum2.Metric5xxError, handlerName, 1.0)
                panic(r)
            } else {
                if response.StatusCode >= 400 && response.StatusCode < 500 {
                    _log.Infof("Emitting 4XXError metric")
                    metric.EmitLambdaMetric(enum2.Metric4xxError, handlerName, 1.0)
                } else if response.StatusCode >= 500 {
                    _log.Infof("Emitting 5XXError metric")
                    metric.EmitLambdaMetric(enum2.Metric5xxError, handlerName, 1.0)
                }
            }
        }()

        response, err = handler(ctx, request)
        return response, err
    }
}
