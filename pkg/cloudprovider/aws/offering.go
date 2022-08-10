package aws

type Offering struct {
	capacityType string
	zone         string
	price        float64
}

func (o *Offering) CapacityType() string {
	return o.capacityType
}

func (o *Offering) Zone() string {
	return o.zone
}

func (o *Offering) Price() float64 {
	return o.price
}
