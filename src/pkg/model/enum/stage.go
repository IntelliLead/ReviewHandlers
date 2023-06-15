package enum

type Stage int

const (
    StageLocal Stage = iota
    StageAlpha
    StagaBeta
    StageGamma
    StageProd
)

func (s Stage) String() string {
    return []string{
        "local",
        "alpha",
        "beta",
        "gamma",
        "prod",
    }[s]
}
