package enum

type Metric int

const (
    Metric4xxError Metric = iota
    Metric5xxError
)

func (s Metric) String() string {
    return []string{
        "4XXError",
        "5XXError",
    }[s]
}
