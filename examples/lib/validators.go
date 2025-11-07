package lib

import (
	"fmt"
	"net/url"
)

func ValidateUserRecord(u UserRecord) error {
	if u.Points < 0 {
		return fmt.Errorf("points cannot be negative")
	}
	if u.DemeritPoints != nil && *u.DemeritPoints < 0 {
		return fmt.Errorf("demerit points cannot be negative")
	}
	if u.ProfilePhoto != nil && *u.ProfilePhoto != "" {
		_, err := url.ParseRequestURI(*u.ProfilePhoto)
		if err != nil {
			return fmt.Errorf("invalid profile photo URL")
		}
	}
	return nil
}
