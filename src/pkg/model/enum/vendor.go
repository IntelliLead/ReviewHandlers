package enum

type Vendor int

const (
    VendorGoogle Vendor = iota
)


func (v Vendor) String() string {
    return []string{
        "GOOGLE",
    }[v]
}
