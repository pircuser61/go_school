package pipeline

import "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

type StepHandler interface {
	HandleStep(step *entity.Step) error
}

type MultipleTypesStepHandler struct {
	handlers map[string]StepHandler
}

func (h *MultipleTypesStepHandler) RegisterStepTypeHandler(stepType string, handler StepHandler) {
	h.handlers[stepType] = handler
}

func (h *MultipleTypesStepHandler) HandleStep(step *entity.Step) error {
	handler, ok := h.handlers[step.Type]
	if ok {
		return handler.HandleStep(step)
	}

	return nil
}

func (h *MultipleTypesStepHandler) HandleSteps(steps []*entity.Step) error {
	for _, step := range steps {
		err := h.HandleStep(step)
		if err != nil {
			return err
		}
	}

	return nil
}
