package util

import (
    "github.com/go-playground/validator/v10"
    "time"
)

func LastRepliedReplyValidation(fl validator.FieldLevel) bool {
    field := fl.Field()

    // Check if both LastReplied and Reply fields are either both nil or both non-nil
    if field.Index(0).IsZero() != field.Index(1).IsZero() {
        return false
    }

    // If non-nil, validate the pointer values
    if !field.Index(0).IsZero() {
        lastReplied, ok := field.Index(0).Interface().(time.Time)
        if !ok {
            return false
        }

        _, ok = field.Index(1).Interface().(string)
        if !ok {
            return false
        }

        currentTime := time.Now()
        if lastReplied.After(currentTime) {
            return false
        }
    }

    return true
}
