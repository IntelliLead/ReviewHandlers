package enum

type Action int

const (
    ActionUpdate Action = iota
    ActionDelete Action = iota
)

func (v Action) String() string {
    return []string{
        "UPDATE",
        "DELETE",
    }[v]
}
