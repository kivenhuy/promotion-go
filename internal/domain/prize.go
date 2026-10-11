package domain

import "errors"

type Prize struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Quantity   int    `json:"-"`
	Weight     int    `json:"-"`
	MaxPerUser int    `json:"-"`
}

func NewPrize(id int, name string, quantity int, weight int) (*Prize, error) {
	p := &Prize{
		ID:       id,
		Name:     name,
		Quantity: quantity,
		Weight:   weight,
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Prize) Validate() error {
	if p.ID <= 0 {
		return errors.New("id must be greater than 0")
	}
	if p.Name == "" {
		return errors.New("name cannot be empty")
	}
	if p.Quantity < 0 {
		return errors.New("quantity must not be negative")
	}
	if p.Weight <= 0 {
		return errors.New("weight must be greater than 0")
	}
	return nil
}

func (p *Prize) Available() bool {
	return p.Quantity > 0
}

func (p *Prize) Take() error {
	if !p.Available() {
		return errors.New("prize is not available")
	}
	p.Quantity--
	return nil
}
