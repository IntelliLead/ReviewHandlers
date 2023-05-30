package model

type SubscriptionTier int

const (
    SubscriptionTierBeta SubscriptionTier = iota
)


func (s SubscriptionTier) String() string {
    return []string{
        "BETA",
    }[s]
}
