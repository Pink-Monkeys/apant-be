package db

import (
	"errors"

	"gorm.io/gorm"
)

func isNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
