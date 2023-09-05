package middleware

import (
    "context"
    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/cloudwatch"
    "log"
)

func MetricMiddleware(handler func(context.Context, events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error)) func(context.Context, events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
    return func(ctx context.Context, request events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
        response, err := handler(ctx, request)

        // Emit custom metrics based on the response status code
        if response.StatusCode >= 400 && response.StatusCode < 500 {
            emitMetric("4XXError", 1.0)
        } else if response.StatusCode >= 500 {
            emitMetric("5XXError", 1.0)
        }

        return response, err
    }
}

func emitMetric(metricName string, value float64) {
    svc := cloudwatch.New(session.New())
    _, err := svc.PutMetricData(&cloudwatch.PutMetricDataInput{
        Namespace: aws.String("AWS/Lambda"),
        MetricData: []*cloudwatch.MetricDatum{
            {
                MetricName: aws.String(metricName),
                Unit:       aws.String("Count"),
                Value:      aws.Float64(value),
            },
        },
    })
    if err != nil {
        log.Println("Error emitting metric: ", err)
    }
}
