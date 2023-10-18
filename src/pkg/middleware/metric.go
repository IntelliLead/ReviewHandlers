package middleware

import (
    "context"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
    "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

var (
    _log = logger.NewLogger()
)

func MetricMiddleware(handlerName string,
    handler func(context.Context, events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error)) func(context.Context, events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    return func(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
        var response events.LambdaFunctionURLResponse
        var err error

        // Use defer to ensure metric emission even in case of panic
        defer func() {
            r := recover()
            if r != nil {
                _log.Infof("Emitting 5XXError metric due to panic")
                emitMetric("5XXError", handlerName, 1.0)
                panic(r)
            } else {
                if response.StatusCode >= 400 && response.StatusCode < 500 {
                    _log.Infof("Emitting 4XXError metric")
                    emitMetric("4XXError", handlerName, 1.0)
                } else if response.StatusCode >= 500 {
                    _log.Infof("Emitting 5XXError metric")
                    emitMetric("5XXError", handlerName, 1.0)
                }
            }
        }()

        response, err = handler(ctx, request)
        return response, err
    }
}

func emitMetric(metricName string, handlerName string, value float64) {
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-northeast-1"))
    if err != nil {
        _log.Error("Error loading AWS config: ", err)
    }
    svc := cloudwatch.NewFromConfig(cfg)
    _, err = svc.PutMetricData(context.TODO(), &cloudwatch.PutMetricDataInput{
        Namespace: aws.String("AWS/Lambda"),
        MetricData: []types.MetricDatum{
            {
                MetricName: aws.String(metricName),
                Dimensions: []types.Dimension{
                    {
                        Name:  aws.String("FunctionName"),
                        Value: aws.String(handlerName),
                    },
                },
                Unit:  types.StandardUnitCount,
                Value: aws.Float64(value),
            },
        },
    })
    if err != nil {
        _log.Error("Error emitting metric: ", err)
    }
}
