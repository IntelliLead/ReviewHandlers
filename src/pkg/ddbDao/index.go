package ddbDao

type UserIndex int

const (
    UserIndexCreatedAtLsi UserIndex = iota
    UserIndexLastUpdatedLsi
)

func (i UserIndex) String() string {
    return []string{
        "createdAt-lsi",
        "lastUpdated-lsi",
    }[i]
}

type ReviewIndex int

const (
    ReviewIndexCreatedAtLsi ReviewIndex = iota
    ReviewIndexLastRepliedLsi
    ReviewIndexLastUpdatedLsi
    ReviewIndexNumberRatingLsi
    ReviewIndexReviewLastUpdatedLsi
)

func (i ReviewIndex) String() string {
    return []string{
        "createdAt-lsi",
        "lastReplied-lsi",
        "numberRating-lsi",
        "reviewLastUpdated-lsi",
    }[i]
}
