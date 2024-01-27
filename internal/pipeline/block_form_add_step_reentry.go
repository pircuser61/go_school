package pipeline

import "context"

func (gb *GoFormBlock) addStepWithReentry(ctx context.Context, reEntry bool) error {
	if reEntry {
		if err := gb.reEntry(ctx); err != nil {
			return err
		}

		gb.RunContext.VarStore.AddStep(gb.Name)
	}

	return nil
}
