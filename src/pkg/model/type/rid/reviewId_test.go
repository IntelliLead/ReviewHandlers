package rid

import (
    "testing"
)

func TestNewReviewId(t *testing.T) {
    id, _ := NewReviewId("az")
    expected := "097122"

    if id.NumericString() != expected {
        t.Errorf("Expected %s, but got %s", expected, id)
    }

    id, err := NewReviewId("@a")
    if err == nil {
        t.Errorf("Expected error, but got %s", id)
    }
}

func TestReviewId_String(t *testing.T) {
    id, _ := NewReviewId("az")
    expected := "az"
    result := id.String()

    if result != expected {
        t.Errorf("Expected %s, but got %s", expected, result)
    }
}

func TestReviewId_NumericString(t *testing.T) {
    id, _ := NewReviewId("az")
    expected := "097122"
    result := id.NumericString()

    if result != expected {
        t.Errorf("Expected %s, but got %s", expected, result)
    }
}

func TestReviewId_Numeric(t *testing.T) {
    id, _ := NewReviewId("az")
    expected := 97122
    result, _ := id.Numeric()

    if result != expected {
        t.Errorf("Expected %v, but got %v", expected, result)
    }
}

func TestReviewId_GetNext(t *testing.T) {
    testCases := []struct {
        input    string
        expected string
    }{
        // single-char
        {"0", "1"},
        {"9", "A"},
        {"Z", "a"},
        // multi-char
        {"abc", "abd"},
        {"xyz0", "xyz1"},
        // carry
        {"az", "b0"},
        {"azz", "b00"},
        {"z", "00"},
        {"zzz", "0000"},
    }

    for _, tc := range testCases {
        id, _ := NewReviewId(tc.input)
        result := id.GetNext().String()

        if result != tc.expected {
            t.Errorf("For input %s, expected %s, but got %s", tc.input, tc.expected, result)
        }
    }
}
