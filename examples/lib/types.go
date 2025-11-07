package lib

// Define your record type
type UserRecord struct {
	Email         string  `csv:"email"`
	Name          string  `csv:"name"`
	Points        int     `csv:"points"`
	DemeritPoints *int    `csv:"demerit_points"`
	ProfilePhoto  *string `csv:"profile_photo"`
}
