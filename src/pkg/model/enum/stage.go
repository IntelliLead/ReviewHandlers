package enum

import (
    "log"
    "strings"
)

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

var (
    stageMap = map[string]Stage{
        "local": StageLocal,
        "alpha": StageAlpha,
        "beta":  StagaBeta,
        "gamma": StageGamma,
        "prod":  StageProd,
    }
)

func StringToStage(str string) Stage {
    c, ok := stageMap[strings.ToLower(str)]
    if !ok {
        log.Fatal("Invalid stage: ", str)
    }

    return c
}
