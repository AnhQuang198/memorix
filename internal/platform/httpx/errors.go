package httpx

type ErrorCode string

const (
	CodeValidation      ErrorCode = "VALIDATION_ERROR"
	CodeUnauthenticated ErrorCode = "UNAUTHENTICATED"
	CodeForbidden       ErrorCode = "FORBIDDEN"
	CodeNotFound        ErrorCode = "NOT_FOUND"
	CodeConflict        ErrorCode = "CONFLICT"
	CodeUnprocessable   ErrorCode = "UNPROCESSABLE"
	CodeRateLimited     ErrorCode = "RATE_LIMITED"
	CodeInternal        ErrorCode = "INTERNAL"
)

// APIError là envelope lỗi chuẩn (AD-14). Marshal thành {"error":{...}}.
type APIError struct {
	Code    ErrorCode         `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	TraceID string            `json:"trace_id,omitempty"`
}

func NewError(code ErrorCode, msg string) *APIError {
	return &APIError{Code: code, Message: msg}
}

func (e *APIError) WithField(k, v string) *APIError {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	e.Fields[k] = v
	return e
}

func (e *APIError) WithTrace(id string) *APIError { e.TraceID = id; return e }

func (e *APIError) Error() string { return string(e.Code) + ": " + e.Message }

func (e *APIError) MarshalJSON() ([]byte, error) {
	type inner APIError
	return jsonMarshal(map[string]inner{"error": inner(*e)})
}

func (e *APIError) HTTPStatus() int {
	switch e.Code {
	case CodeValidation, CodeUnprocessable:
		if e.Code == CodeUnprocessable {
			return 422
		}
		return 400
	case CodeUnauthenticated:
		return 401
	case CodeForbidden:
		return 403
	case CodeNotFound:
		return 404
	case CodeConflict:
		return 409
	case CodeRateLimited:
		return 429
	default:
		return 500
	}
}
