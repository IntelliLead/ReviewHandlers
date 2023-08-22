package _type

import (
    "fmt"
    "github.com/aws/aws-sdk-go/service/dynamodb"
    "strconv"
    "time"
)

type EpochMs time.Time

// must use value receiver so that DDB Go SDK is able to properly marshal/unmarshal
func (e EpochMs) MarshalDynamoDBAttributeValue(av *dynamodb.AttributeValue) error {
    millis := TimeToMilli(time.Time(e))
    millisStr := fmt.Sprintf("%d", millis)
    av.N = &millisStr
    return nil
}

func (e *EpochMs) UnmarshalDynamoDBAttributeValue(av *dynamodb.AttributeValue) error {
    millis, err := strconv.ParseInt(*av.N, 10, 0)
    if err != nil {
        return err
    }
    *e = EpochMs(MilliToTime(millis))
    return nil
}

func TimeToMilli(t time.Time) int64 {
    nanosSinceEpoch := t.UnixNano()
    return (nanosSinceEpoch / 1_000_000_000) + (nanosSinceEpoch % 1_000_000_000)
}

func MilliToTime(millis int64) time.Time {
    seconds := millis / 1_000
    nanos := (millis % 1_000) * 1_000_000
    return time.Unix(seconds, nanos)
}

// String calls the underlying time.Time.String to return a human-readable representation
func (e EpochMs) String() string {
    return time.Time(e).String()
}
