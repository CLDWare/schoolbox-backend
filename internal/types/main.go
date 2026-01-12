package apiResponses

type BaseBase struct {
	Status    int    `example:"200"`
	Success   bool   `example:"true"`
	Message   string `example:"Ok"`
	Timestamp string `example:"2026-01-12T21:52:50.253429709+01:00" format:"date-time"`
}

type BaseResponse struct {
	BaseBase
	Data any
}

type BaseError struct {
	BaseBase
}

type BadRequestError struct {
	BaseBase
	Status  int    `default:"400"`
	Success bool   `default:"false"`
	Message string `example:"Could not parse 'hi' as int"`
}
type UnauthorizedError struct {
	BaseBase
	Status  int    `default:"401"`
	Success bool   `default:"false"`
	Message string `example:"Invalid or expired session"`
}
type ForbiddenError struct {
	BaseBase
	Status  int    `default:"403"`
	Success bool   `default:"false"`
	Message string `default:"Forbidden"`
}
type NotFoundError struct {
	BaseBase
	Status  int    `default:"404"`
	Success bool   `default:"false"`
	Message string `default:"Not Found"`
}

type InternalServerError struct {
	BaseBase
	Status  int    `default:"500"`
	Success bool   `default:"false"`
	Message string `default:"Internal Server Error"`
}
