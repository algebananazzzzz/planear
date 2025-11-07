package lib

func ptrInt(i int) *int          { return &i }
func ptrString(s string) *string { return &s }

// Mock function to load remote records (simulate DB)
func LoadRemoteRecords() (map[string]UserRecord, error) {

	return map[string]UserRecord{
		"user1@example.com": {
			Email:         "user1@example.com",
			Name:          "Alice",
			Points:        100,
			DemeritPoints: nil,
			ProfilePhoto:  ptrString("https://example.com/photo1.jpg"),
		},
		"user2@example.com": {
			Email:         "user2@example.com",
			Name:          "Bob",
			Points:        50, // changed in CSV to 60 (update)
			DemeritPoints: ptrInt(4),
			ProfilePhoto:  ptrString("https://example.com/photo2.jpg"),
		},
		"user3@example.com": {
			Email:         "user3@example.com",
			Name:          "Charlie",
			Points:        75,
			DemeritPoints: nil,
			ProfilePhoto:  ptrString("https://example.com/photo3.jpg"),
		},
		"user4@example.com": {
			Email:         "user4@example.com",
			Name:          "Diana",
			Points:        0,
			DemeritPoints: ptrInt(1), // CSV has -1 (ignore)
			ProfilePhoto:  ptrString("https://validurl.com/photo4.jpg"),
		},
		"user6@example.com": {
			Email:         "user6@example.com",
			Name:          "Frank",
			Points:        10, // CSV negative points (-5), ignore
			DemeritPoints: ptrInt(0),
			ProfilePhoto:  ptrString("https://example.com/photo6.jpg"),
		},
		"user7@example.com": {
			Email:         "user7@example.com",
			Name:          "Gina",
			Points:        40,
			DemeritPoints: nil,
			ProfilePhoto:  ptrString("https://example.com/photo7.jpg"),
		},
	}, nil
}
