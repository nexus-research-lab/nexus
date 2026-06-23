package provider

func (r *Repository) bind(index int) string {
	return r.dialect.Bind(index)
}

func (r *Repository) trueValue() string {
	return r.dialect.TrueValue()
}

func (r *Repository) falseValue() string {
	return r.dialect.FalseValue()
}

func (r *Repository) currentTimestamp() string {
	return r.dialect.CurrentTimestamp()
}
