package ddbDao

type Table int

const (
    USER Table = iota
    REVIEW
)


func (t Table) String() string {
    return []string{"User", "Review"}[t]
}
