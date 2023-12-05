package util

func GetToggleUrl(state bool) string {
    if state {
        return ToggleOnFlexMessageImageUrl
    }
    return ToggleOffFlexMessageImageUrl
}
