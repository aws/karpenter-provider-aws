package metadata

type Info struct {
	region    string
	accountID string
}

func NewInfo(region, accountID string) *Info {
	return &Info{
		region:    region,
		accountID: accountID,
	}
}

func (i *Info) Region() string {
	return i.region
}

func (i *Info) AccountID() string {
	return i.accountID
}
