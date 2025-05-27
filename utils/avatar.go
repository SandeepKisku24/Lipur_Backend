package utils

func GenerateAvatar(title, artist string) string {
	// For MVP, just return initials or dummy avatar url
	return "https://dummyimage.com/100x100/000/fff&text=" + string(title[0]) + string(artist[0])
}
