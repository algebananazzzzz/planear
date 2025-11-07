package lib

import "fmt"

func FormatRecord(r UserRecord) string {
	demerit := "nil"
	if r.DemeritPoints != nil {
		demerit = fmt.Sprintf("%d", *r.DemeritPoints)
	}
	photo := "nil"
	if r.ProfilePhoto != nil {
		photo = *r.ProfilePhoto
	}
	return fmt.Sprintf("Email: %s, Name: %s, Points: %d, DemeritPoints: %s, ProfilePhoto: %s",
		r.Email, r.Name, r.Points, demerit, photo)
}

func FormatKey(key string) string {
	return fmt.Sprintf("Email: %s", key)
}

func ExtractKey(r UserRecord) string {
	return r.Email
}
