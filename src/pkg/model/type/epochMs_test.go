package _type

import (
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "testing"
    "time"
)

var (
    // 1692596673000 is (GMT) Monday, August 21, 2023 5:44:33 AM
    // create new time.Time from epochMs
    millis   = int64(1692596673000)
    timeTime = time.Date(2023, 8, 21, 5, 44, 33, 0, time.UTC)
    // create new EpochMs from time.Time
    epochMs = EpochMs(timeTime)
)

func TestEpochMsString(t *testing.T) {
    if epochMs.String() != "2023-08-21 05:44:33 +0000 UTC" {
        t.Errorf("expected epochMs to be 2023-08-21 05:44:33 +0000 UTC, got %s", epochMs.String())
    }
}

func TestEpochMsToMilli(t *testing.T) {
    if TimeToMilli(timeTime) != millis {
        t.Errorf("expected TimeToMilli to be %d, got %d", millis, TimeToMilli(timeTime))
    }
}

func TestMilliToEpochMs(t *testing.T) {
    if MilliToTime(millis) != timeTime {
        t.Errorf("expected MilliToTime to be %s, got %s", timeTime, MilliToTime(millis))
    }
}

func TestEpochMsMarshalDynamoDBAttributeValue(t *testing.T) {
    av := &dynamodb.AttributeValue{}
    err := epochMs.MarshalDynamoDBAttributeValue(av)
    if err != nil {
        t.Errorf("expected err to be nil, got %v", err)
    }
    if *av.N != "1692596673" {
        t.Errorf("expected av.N to be 1692596673, got %s", *av.N)
    }
}

func TestEpochMsUnmarshalDynamoDBAttributeValue(t *testing.T) {
    millisStr := "1692596673000"
    av := &dynamodb.AttributeValue{N: &millisStr}
    err := epochMs.UnmarshalDynamoDBAttributeValue(av)
    if err != nil {
        t.Errorf("expected err to be nil, got %v", err)
    }
    if epochMs != epochMs {
        t.Errorf("expected epochMs to be %s, got %s", epochMs, epochMs)
    }
}
