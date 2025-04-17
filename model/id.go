package model

type (
	ID interface {
		Aid | AidX
	}
	Aid  string
	AidX string
)
