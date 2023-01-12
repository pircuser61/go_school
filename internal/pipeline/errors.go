package pipeline

type UserIsNotPartOfProcessErr struct{}

func NewUserIsNotPartOfProcessErr() error {
	return UserIsNotPartOfProcessErr{}
}

func (e UserIsNotPartOfProcessErr) Error() string {
	return "user is not part of the process"
}

func (e UserIsNotPartOfProcessErr) Is(err error) bool {
	_, ok := err.(UserIsNotPartOfProcessErr)
	return ok
}
