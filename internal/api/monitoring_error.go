package api

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

func (ae *Env) MonitoringGetBlockError(w http.ResponseWriter, r *http.Request, blockID string) {
	ctx, span := trace.StartSpan(r.Context(), "monitoring_get_block_error")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("stepID", blockID)
	errorHandler := newHTTPErrorHandler(log, w)

	blockIsHidden, err := ae.DB.CheckBlockForHiddenFlag(ctx, blockID)
	if err != nil {
		e := CheckForHiddenError
		log.Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	if blockIsHidden {
		errorHandler.handleError(ForbiddenError, nil)

		return
	}

	blockIDUUID, err := uuid.Parse(blockID)
	if err != nil {
		errorHandler.handleError(UUIDParsingError, err)
	}

	taskStep, err := ae.DB.GetTaskStepByID(ctx, blockIDUUID)
	if err != nil {
		e := UnknownError

		log.Error(e.errorMessage(err))
		errorHandler.sendError(e)
	}

	desc := fmt.Sprintf(ae.getErrorDescription(), blockID, taskStep.WorkNumber)
	urlError := ae.GetErrorURL(taskStep.WorkNumber, blockID)

	if err = sendResponse(w, http.StatusOK, BlockErrorResponse{
		Description: desc,
		Url:         urlError,
	}); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:all // ok
func (ae *Env) getErrorDescription() string {
	return `Для просмотра ошибок по данному блоку: 
	1. Получите права на доступ к индексу Jocasta на https://dashboards.obs.mts.ru/, для этого можно обратиться к Быкову Сергею (svbyk11@mts.ru), Нуриеву Денису (dgnuriy1@mts.ru)
	2. Войдите на https://dashboards.obs.mts.ru/
	3. Произведите выборку записей по фильтрам
		- stepID = %s
		- workNumber = %s		
		- method oneOf(POST, PUT, kafka, faas) 
		- level = error
	или воспользуйтесь предлагаемой ссылкой`
}

//nolint:all // ok
func (ae *Env) GetErrorURL(workNumber, stepID string) string {
	const (
		logRequestStart           = `https://dashboards.obs.mts.ru/app/data-explorer/discover#?_a=(discover:(columns:!(_source),isDirty:!f,sort:!()),metadata:(indexPattern:%s,view:discover))`
		logRequestStart2          = `&_g=(filters:!(),refreshInterval:(pause:!t,value:0),time:(from:now-20d,to:now))&_q=(filters:!(`
		logRequestFilter          = `('$state':(store:appState),meta:(alias:!n,disabled:!f,index:%s,key:%s,negate:!f,params:(query:'%s'),type:phrase),query:(match_phrase:(%s:'%s'))),`
		logRequestFilterMethod    = `('$state':(store:appState),meta:(alias:!n,disabled:!f,index:%s,key:method,negate:!f,params:!(POST,PUT,kafka,faas),type:phrases,value:'POST,PUT,kafka,faas'),`
		logRequestFilterMethodEnd = `query:(bool:(minimum_should_match:1,should:!((match_phrase:(method:POST)),(match_phrase:(method:PUT)),(match_phrase:(method:kafka)),(match_phrase:(method:faas))))))),`
		logRequestEnd             = `query:(language:kuery,query:''))`
	)

	indexJocasta := ae.LogIndex

	return fmt.Sprintf(logRequestStart, indexJocasta) + logRequestStart2 +
		fmt.Sprintf(logRequestFilter, indexJocasta, "level", "error", "level", "error") +
		fmt.Sprintf(logRequestFilter, indexJocasta, "stepID", stepID, "stepID", stepID) +
		fmt.Sprintf(logRequestFilter, indexJocasta, "workNumber", workNumber, "workNumber", workNumber) +
		fmt.Sprintf(logRequestFilterMethod, indexJocasta) + logRequestFilterMethodEnd +
		logRequestEnd
}
