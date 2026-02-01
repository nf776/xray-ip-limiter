package repository

type LimitsRepository interface {
	GetLimit(userID int) (int, bool)
	UpdateLimits(limits map[int]int)
}
