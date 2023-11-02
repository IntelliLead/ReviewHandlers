package rid

import (
    "fmt"
    "github.com/go-playground/validator/v10"
    "strconv"
)

// See ReviewId design
// https://www.notion.so/Engineering-Low-Level-Design-1f52a247123e4e42824482d7ab6de831?pvs=4#c993f9741b414db990b23f09cf2699e8
type ReviewId string

var validate *validator.Validate

func init() {
    validate = validator.New(validator.WithRequiredStructEnabled())
}

func NewReviewId(alphanumeric string) (ReviewId, error) {
    if err := validate.Var(alphanumeric, "alphanum"); err != nil {
        return "", err
    }

    var idStr string
    for _, char := range alphanumeric {
        idStr += fmt.Sprintf("%03s", strconv.Itoa(int(char)))
    }

    return ReviewId(idStr), nil
}

// String returns the string representation of a ReviewId
// e.g. returned.String() == "az"
func (id ReviewId) String() string {
    var alphanumeric string
    for i := 0; i < len(id); i += 3 {
        asciiCode, _ := strconv.Atoi(string(id[i : i+3]))
        char := []rune{rune(asciiCode)}
        alphanumeric += string(char)
    }

    return alphanumeric
}

// NumericString returns the numeric representation of a ReviewId in string type
// e.g. NewReviewId("az") -> returned.NumericString() == "097122"
func (id ReviewId) NumericString() string {
    return string(id)
}

// Numeric returns the numeric representation of a ReviewId in int type
// e.g. NewReviewId("az") -> returned.Numeric() == 97122    // leading 0 is omitted
func (id ReviewId) Numeric() (int, error) {
    parseInt, err := strconv.ParseInt(id.NumericString(), 10, 0)
    return int(parseInt), err
}

func (id ReviewId) GetNext() ReviewId {
    numericStr := id.NumericString()                                         // e.g. "097122"
    newNumericStr, _ := incrementAsciiCodes(numericStr, len(numericStr)/3-1) // "097122", 1
    return ReviewId(newNumericStr)
}

func incrementAsciiCodes(numericStr string, idx int) (string, bool) {
    if idx < 0 {
        return fmt.Sprintf("%03s", strconv.Itoa(48)) + numericStr, true // ASCII code for '0'
    }

    asciiCode, _ := strconv.Atoi(numericStr[idx*3 : idx*3+3]) // [3: 6]
    newAsciiCode, carry := getNextAsciiCode(asciiCode)
    if carry {
        newNumericStr, _ := incrementAsciiCodes(numericStr[:idx*3], idx-1)
        return newNumericStr + fmt.Sprintf("%03s", strconv.Itoa(newAsciiCode)), carry
    }
    return numericStr[:idx*3] + fmt.Sprintf("%03s", strconv.Itoa(newAsciiCode)) + numericStr[idx*3+3:], carry
}

// getNextAsciiCode returns the next ascii code and a bool indicating if a carry occurred (exhausted all ascii codes)
func getNextAsciiCode(lastAsciiCode int) (int, bool) {
    var nextAsciiCode int
    switch lastAsciiCode {
    case 57: // ASCII code for '9'
        nextAsciiCode = 65 // ASCII code for 'A'
    case 90: // ASCII code for 'Z'
        nextAsciiCode = 97 // ASCII code for 'a'
    case 122: // ASCII code for 'z'
        return 48, true // ASCII code for '0'
    default:
        nextAsciiCode = lastAsciiCode + 1 // ASCII code for 'a'
    }

    return nextAsciiCode, false
}

func ReviewIdPtrNumericValidation(fl validator.FieldLevel) bool {
    reviewId := fl.Field().Interface().(*ReviewId)

    // Check if UserReviewId is nil
    if reviewId == nil {
        return true
    }

    // Check if UserReviewId is a numbers-only string
    if !isNumbersOnly(string(*reviewId)) {
        return false
    }

    // Check if UserReviewId length is divisible by 3
    if len(*reviewId)%3 != 0 {
        return false
    }

    return true
}
func ReviewIdNumericValidation(fl validator.FieldLevel) bool {
    reviewId := fl.Field().Interface().(ReviewId)

    // Check if UserReviewId is a numbers-only string
    if !isNumbersOnly(string(reviewId)) {
        return false
    }

    // Check if UserReviewId length is divisible by 3
    if len(reviewId)%3 != 0 {
        return false
    }

    return true
}

func isNumbersOnly(s string) bool {
    for _, c := range s {
        if c < '0' || c > '9' {
            return false
        }
    }
    return true
}
