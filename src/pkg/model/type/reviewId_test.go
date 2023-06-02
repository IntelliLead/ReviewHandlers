package _type

import (
    "testing"
)

func TestNewReviewId(t *testing.T) {
    id := NewReviewId("az")
    expected := "097122"

    if id.Numeric() != expected {
        t.Errorf("Expected %s, but got %s", expected, id)
    }
}

func TestReviewId_String(t *testing.T) {
    id := NewReviewId("az")
    expected := "az"
    result := id.String()

    if result != expected {
        t.Errorf("Expected %s, but got %s", expected, result)
    }
}

func TestReviewId_GetNext(t *testing.T) {
    testCases := []struct {
        input    string
        expected string
    }{
        {"az", "az0"},
        {"0", "1"},
        {"9", "A"},
        {"Z", "a"},
        {"z", "z0"},
        {"abc", "abd"},
        {"xyz0", "xyz1"},
    }

    for _, tc := range testCases {
        id := NewReviewId(tc.input)
        result := id.GetNext().String()

        if result != tc.expected {
            t.Errorf("For input %s, expected %s, but got %s", tc.input, tc.expected, result)
        }
    }
}
