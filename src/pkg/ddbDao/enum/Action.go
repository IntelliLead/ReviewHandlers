package enum

type Action int

const (
    ActionUpdate Action = iota
    ActionRemove Action = iota
    ActionAppend Action = iota // used for appending elements in a list
)

func (v Action) String() string {
    return []string{
        "UPDATE",
        "REMOVE",
        "APPEND",
    }[v]
}
