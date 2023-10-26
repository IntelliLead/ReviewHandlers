package enum

type Metric int

const (
    Metric4xxError Metric = iota
    Metric5xxError
    MetricMultipleBusinessAccounts
    MetricMultipleBusinessLocations
)

func (s Metric) String() string {
    return []string{
        "4XXError",
        "5XXError",
        "MultipleBusinessAccounts",
        "MultipleBusinessLocations",
    }[s]
}
