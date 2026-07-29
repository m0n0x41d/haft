package profileprojection

func SetIdentifierSourceForTest(
	service *Service,
	source func(string) (string, error),
) {
	service.newIdentifier = source
}

func RandomIdentifierForTest(prefix string) (string, error) {
	return randomIdentifier(prefix)
}
