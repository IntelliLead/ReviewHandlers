// package dbModel
//
// type UniqueLineUser struct {
// 	LineUserId string // partition key
// 	UserId     string // sort key
// }
//
// const UniqueLineUserIdPrefix = "#UNIQUE_LINE_USER_ID#"
//
// func NewUniqueLineUserId(lineUserId string, userId string) *UniqueLineUser {
// 	return &UniqueLineUser{
// 		LineUserId: UniqueLineUserIdPrefix + lineUserId,
// 		UserId:     userId,
// 	}
// }
