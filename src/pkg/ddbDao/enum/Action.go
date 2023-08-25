package enum

type Action int

const (
    ActionUpdate          Action = iota
    ActionRemove          Action = iota
    ActionAppendStringSet Action = iota // used for appending elements in a string set
)

func (v Action) String() string {
    return []string{
        "UPDATE",
        "REMOVE",
        "APPEND",
    }[v]
}
