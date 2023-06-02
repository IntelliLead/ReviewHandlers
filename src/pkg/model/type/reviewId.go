package _type

import (
    "fmt"
    "strconv"
)

// See ReviewId design
// https://www.notion.so/Engineering-Low-Level-Design-1f52a247123e4e42824482d7ab6de831?pvs=4#c993f9741b414db990b23f09cf2699e8
type ReviewId string

func NewReviewId(alphanumeric string) ReviewId {
    var idStr string
    for _, char := range alphanumeric {
        idStr += fmt.Sprintf("%03s", strconv.Itoa(int(char)))
    }

    return ReviewId(idStr)
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

// Numeric returns the numeric representation of a ReviewId in string type
// e.g. NewReviewId("az") -> returned.Numeric() == "097122"
func (id ReviewId) Numeric() string {
    return string(id)
}

func (id ReviewId) GetNext() ReviewId {
    numeric := id.Numeric()

    // get last ascii code
    lastAsciiCode, _ := strconv.Atoi(numeric[len(numeric)-3:])
    issueNewChar := false
    var newLastAsciiCode int
    switch lastAsciiCode {
    case 57: // ASCII code for '9'
        newLastAsciiCode = 65 // ASCII code for 'A'
    case 90: // ASCII code for 'Z'
        newLastAsciiCode = 97 // ASCII code for 'a'
    case 122: // ASCII code for 'z'
        issueNewChar = true
    default:
        newLastAsciiCode = lastAsciiCode + 1 // ASCII code for 'a'
    }

    if issueNewChar {
        return ReviewId(numeric + "048") // ASCII code for '0'
    }

    return ReviewId(numeric[:len(numeric)-3] + fmt.Sprintf("%03s", strconv.Itoa(newLastAsciiCode)))
}

func (id ReviewId) GetPrevious() ReviewId {
    println("Calling GetPrevious() on ", id.String())

    numeric := id.Numeric()

    // get last ascii code
    lastAsciiCode, _ := strconv.Atoi(numeric[len(numeric)-3:])
    var newLastAsciiCode int
    switch lastAsciiCode {
    case 48: // ASCII code for '0'
        // trim last 3 numbers (last ascii char)
        return ReviewId(numeric[:len(numeric)-3])
    case 65: // ASCII code for 'A'
        newLastAsciiCode = 57 // ASCII code for '9'
    case 97: // ASCII code for 'a'
        newLastAsciiCode = 90 // ASCII code for 'Z'
    default:
        newLastAsciiCode = lastAsciiCode - 1
    }

    println("DEBUG: new ascii code str: ", numeric[:len(numeric)-3]+fmt.Sprintf("%03s", strconv.Itoa(newLastAsciiCode)))
    return ReviewId(numeric[:len(numeric)-3] + fmt.Sprintf("%03s", strconv.Itoa(newLastAsciiCode)))
}
