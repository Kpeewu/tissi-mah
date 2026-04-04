package errors

import "errors"

var (
	ErrorNotFound          = errors.New("ErrorNotFound")
	ErrorInvalidInput      = errors.New("ErrorInvalidInput")
	ErrorDuplicate         = errors.New("ErrorDuplicate")
	ErrorInternalServer    = errors.New("ErrorInternalServer")
	ErrorDataRetrievalFail = errors.New("ErrorDataRetrievalFail")
	ErrorDataInsertFail    = errors.New("ErrorDataInsertFail")
	ErrorDataUpdateFail    = errors.New("ErrorDataUpdateFail")
	ErrorDataDeleteFail    = errors.New("ErrorDataDeleteFail")
	ErrorTemplateNotFound  = errors.New("ErrorTemplateNotFound")
	ErrorRoutingNotFound   = errors.New("ErrorRoutingNotFound")
)
