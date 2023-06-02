package enum

type Table int

const (
    TableUser Table = iota
    TableReview
)

func (t Table) String() string {
    return []string{"User", "Review"}[t]
}
