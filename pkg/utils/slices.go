package utils

func Contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func AppendIfNotPresent(slice []string, item string) []string {
	if !Contains(slice, item) {
		slice = append(slice, item)
	}
	return slice
}
