// Package employeev1 mimics protoc-generated wire types (open API
// style: exported fields plus nil-safe getters).
package employeev1

import "google.golang.org/genproto/googleapis/type/date"

// Employee is the wire-side representation of model.Employee.
type Employee struct {
	state     int
	Id        string
	Name      string
	HiredAt   *date.Date
	Address   *Address
	Nicknames []string
}

func (x *Employee) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Employee) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Employee) GetHiredAt() *date.Date {
	if x != nil {
		return x.HiredAt
	}
	return nil
}

func (x *Employee) GetAddress() *Address {
	if x != nil {
		return x.Address
	}
	return nil
}

func (x *Employee) GetNicknames() []string {
	if x != nil {
		return x.Nicknames
	}
	return nil
}

// Address is the wire-side representation of model.Address.
type Address struct {
	state  int
	City   string
	Street string
}

func (x *Address) GetCity() string {
	if x != nil {
		return x.City
	}
	return ""
}

func (x *Address) GetStreet() string {
	if x != nil {
		return x.Street
	}
	return ""
}
