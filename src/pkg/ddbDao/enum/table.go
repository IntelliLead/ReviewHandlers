package enum

type Table int

const (
    TableUser Table = iota
    TableReview
    TableBusiness
)

func (t Table) String() string {
    return []string{
        "User",
        "Review",
        "Business",
    }[t]
}
