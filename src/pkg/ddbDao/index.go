package ddbDao

type UserIndex int

const (
	LINE_USER_ID_USER_ID_GSI UserIndex = iota
	LAST_UPDATED_LSI
)

func (i UserIndex) String() string {
	return []string{
		"lineUserId-userId-gsi",
		"lastUpdatedLSI",
	}[i]
}


type ReviewIndex int

const (
	LINE_USER_ID_REVIEW_ID_GSI ReviewIndex = iota
	CREATED_AT_LSI
	LAST_REPLIED_LSI
	NUMBER_RATING_LSI
)

func (i ReviewIndex) String() string {
	return []string{
		"lineUserId-reviewId-gsi",
		"createdAtLSI",
		"lastRepliedLSI",
		"numberRatingLSI",
	}[i]
}
