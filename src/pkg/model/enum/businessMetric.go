package enum

type BusinessMetric int

const (
    MetricMultipleBusinessAccounts BusinessMetric = iota
    MetricMultipleBusinessLocations
)

func (s BusinessMetric) String() string {
    return []string{
        "MultipleBusinessAccounts",
        "MultipleBusinessLocations",
    }[s]
}
