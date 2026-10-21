package failure

type KindType string

const (
	EmptyValueError         KindType = "empty_value"
	InternalEmptyValueError KindType = "internal_empty_value"
	MalformedError          KindType = "malformed"
	InvalidError            KindType = "invalid"
	GenerationError         KindType = "generation"
	UnauthorizedError       KindType = "unauthorized"
	ForbiddenError          KindType = "forbidden"
	NotFoundError           KindType = "not_found"
	ConflictError           KindType = "conflict"
	GeneralError            KindType = "general"
	InternalGeneralError    KindType = "internal_general"
	RateLimitError          KindType = "rate_limit"
)
