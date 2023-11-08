package metric

import (
    "context"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/logger"
    enum2 "github.com/IntelliLead/ReviewHandlers/src/pkg/metric/enum"
    "github.com/IntelliLead/ReviewHandlers/src/pkg/model/enum"
    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/cloudwatch"
    "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

var (
    _log = logger.NewLogger()
)

func EmitLambdaMetric(metric enum2.Metric, lambdaHandler enum.HandlerName, value float64) {
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-northeast-1"))
    if err != nil {
        _log.Error("Error loading AWS config: ", err)
    }
    svc := cloudwatch.NewFromConfig(cfg)
    _, err = svc.PutMetricData(context.TODO(), &cloudwatch.PutMetricDataInput{
        Namespace: aws.String("AWS/Lambda"),
        MetricData: []types.MetricDatum{
            {
                MetricName: aws.String(metric.String()),
                Dimensions: []types.Dimension{
                    {
                        Name:  aws.String("FunctionName"),
                        Value: aws.String(lambdaHandler.String()),
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

func EmitMetric(metric enum2.Metric, value float64) {
    cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("ap-northeast-1"))
    if err != nil {
        _log.Error("Error loading AWS config: ", err)
    }
    svc := cloudwatch.NewFromConfig(cfg)

    var namespace string
    switch metric {
    case enum2.MetricMultipleBusinessAccounts, enum2.MetricMultipleBusinessLocations:
        namespace = "IntelliLeadAuth/Metrics"
    default:
        namespace = "IntelliLead/Metrics"
    }

    _, err = svc.PutMetricData(context.TODO(), &cloudwatch.PutMetricDataInput{
        Namespace: aws.String(namespace),
        MetricData: []types.MetricDatum{
            {
                MetricName: aws.String(metric.String()),
                Unit:       types.StandardUnitCount,
                Value:      aws.Float64(value),
            },
        },
    })
    if err != nil {
        _log.Error("Error emitting metric: ", err)
    }
}
