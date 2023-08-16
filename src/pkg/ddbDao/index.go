package ddbDao

type UserIndex int

const (
    UserIndexLineUserIdGsi UserIndex = iota
    UserIndexLastUpdatedLsi
)

func (i UserIndex) String() string {
    return []string{
        "lineUserId-userId-gsi",
        "lastUpdatedLSI",
    }[i]
}

type ReviewIndex int

const (
    ReviewIndexLineUserIdReviewIdGsi ReviewIndex = iota
    ReviewIndexCreatedAtLsi
    ReviewIndexLastRepliedLsi
    ReviewIndexNumberRatingLsi
)

func (i ReviewIndex) String() string {
    return []string{
        "lineUserId-reviewId-gsi",
        "createdAtLSI",
        "lastRepliedLSI",
        "numberRatingLSI",
    }[i]
}
