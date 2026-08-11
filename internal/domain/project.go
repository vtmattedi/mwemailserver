package domain

type Project struct {
	ID           int64
	Name         string
	TokenHash    string
	Enabled      bool
	MonthlyLimit *int64
}
